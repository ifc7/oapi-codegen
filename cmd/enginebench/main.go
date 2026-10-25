// Command enginebench times code generation through the kin and libopenapi
// engines on one OpenAPI document.
//
// The default document is written to a temp file: many paths and schemas
// that reference each other, large enough that loading and generating
// dominate the timer. -spec times a file instead. Both engines see the
// same bytes.
//
// Samples alternate which engine runs first. The garbage collector runs
// before each sample so one engine does not pay for the other's garbage.
// Warmup rounds are discarded. The run stops once each engine's 95%
// confidence interval is within -ci of its mean, and always reports that
// interval.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"runtime"
	"text/tabwriter"
	"time"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/libopenapi"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

func main() {
	specPath := flag.String("spec", "", "OpenAPI file to time; empty writes a generated spec")
	resources := flag.Int("resources", 80, "paths in the generated spec, ignored when -spec is set")
	warmupN := flag.Int("warmup", 2, "unmeasured rounds per engine")
	samplesN := flag.Int("samples", 0, "samples per engine; 0 stops once the confidence interval is tight")
	minSamples := flag.Int("min-samples", 12, "minimum samples when -samples is 0")
	maxSamples := flag.Int("max-samples", 30, "maximum samples when -samples is 0")
	ciFrac := flag.Float64("ci", 0.05, "stop when the 95% CI half-width is within this fraction of the mean")
	flag.Parse()

	if flag.NArg() != 0 {
		_, _ = fmt.Fprintf(os.Stderr, "unexpected arguments %q\n", flag.Args())
		os.Exit(2)
	}
	if *warmupN < 0 || *minSamples < 2 || *maxSamples < *minSamples || *ciFrac <= 0 {
		_, _ = fmt.Fprintf(os.Stderr, "warmup must be >= 0, min-samples >= 2, max-samples >= min-samples, and ci > 0\n")
		os.Exit(2)
	}

	path, cleanup, err := openSpec(*specPath, *resources)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "spec: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	info, err := os.Stat(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "spec: %v\n", err)
		os.Exit(1)
	}

	engines := []eng{
		{name: "kin", load: kin.Engine{}.Load},
		{name: "libopenapi", load: libopenapi.Engine{}.Load},
	}
	cfg := codegen.Configuration{
		PackageName: "api",
		Generate: codegen.GenerateOptions{
			Server: true,
			Client: true,
			Models: true,
		},
	}

	_, _ = fmt.Fprintf(os.Stderr, "warming up (%d %s)...\n", *warmupN, plural(*warmupN, "round", "rounds"))
	for round := 0; round < *warmupN; round++ {
		for _, e := range order(engines, round) {
			if _, err := measure(e, path, cfg); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", e.name, err)
				os.Exit(1)
			}
		}
	}

	target := *samplesN
	if target == 0 {
		target = *maxSamples
	}
	taken := make([][]observation, len(engines))
	var docPaths, docOps int
	if *samplesN == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "measuring up to %d samples per engine\n", target)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "measuring %d samples per engine\n", target)
	}
	for n := 0; n < target; n++ {
		for _, idx := range orderIdx(len(engines), n) {
			obs, err := measure(engines[idx], path, cfg)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", engines[idx].name, err)
				os.Exit(1)
			}
			if n == 0 && idx == 0 {
				docPaths, docOps = count(obs.doc)
			}
			obs.doc = nil
			taken[idx] = append(taken[idx], obs)
		}
		_, _ = fmt.Fprintf(os.Stderr, "  sample %d", n+1)
		for i, e := range engines {
			_, _ = fmt.Fprintf(os.Stderr, "  %s %s", e.name, formatDuration(taken[i][n].total))
		}
		_, _ = fmt.Fprintln(os.Stderr)
		if *samplesN == 0 && n+1 >= *minSamples && intervalsTight(taken, *ciFrac) {
			break
		}
	}

	report(os.Stdout, reportInput{
		path:     path,
		bytes:    info.Size(),
		paths:    docPaths,
		ops:      docOps,
		warmup:   *warmupN,
		ciFrac:   *ciFrac,
		engines:  engines,
		samples:  taken,
		explicit: *specPath != "",
	})
}

type eng struct {
	name string
	load func(string, engine.LoadOptions) (*model.Document, error)
}

type observation struct {
	load     time.Duration
	generate time.Duration
	total    time.Duration
	nBytes   int
	sum      [sha256.Size]byte
	doc      *model.Document
}

func measure(e eng, path string, cfg codegen.Configuration) (observation, error) {
	runtime.GC()
	started := time.Now()
	doc, err := e.load(path, engine.LoadOptions{})
	if err != nil {
		return observation{}, fmt.Errorf("load: %w", err)
	}
	loaded := time.Since(started)
	code, err := codegen.Generate(doc, cfg)
	if err != nil {
		return observation{}, fmt.Errorf("generate: %w", err)
	}
	total := time.Since(started)
	return observation{
		load:     loaded,
		generate: total - loaded,
		total:    total,
		nBytes:   len(code),
		sum:      sha256.Sum256([]byte(code)),
		doc:      doc,
	}, nil
}

func openSpec(path string, resources int) (string, func(), error) {
	if path != "" {
		return path, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "enginebench")
	if err != nil {
		return "", nil, err
	}
	written, err := writeSpec(dir, resources)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return written, func() { _ = os.RemoveAll(dir) }, nil
}

func order(engines []eng, round int) []eng {
	out := make([]eng, len(engines))
	for i, idx := range orderIdx(len(engines), round) {
		out[i] = engines[idx]
	}
	return out
}

func orderIdx(n, round int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = (i + round) % n
	}
	return idx
}

func count(doc *model.Document) (paths, ops int) {
	if doc == nil || doc.Paths == nil {
		return 0, 0
	}
	paths = doc.Paths.Len()
	for _, item := range doc.Paths.Map() {
		if item != nil {
			ops += len(item.Operations())
		}
	}
	return paths, ops
}

func intervalsTight(samples [][]observation, frac float64) bool {
	for _, set := range samples {
		s := summarize(totals(set))
		if s.Mean == 0 || float64(s.CI95Half) > frac*float64(s.Mean) {
			return false
		}
	}
	return true
}

func totals(samples []observation) []time.Duration {
	out := make([]time.Duration, len(samples))
	for i, sample := range samples {
		out[i] = sample.total
	}
	return out
}

func column(samples []observation, pick func(observation) time.Duration) []time.Duration {
	out := make([]time.Duration, len(samples))
	for i, sample := range samples {
		out[i] = pick(sample)
	}
	return out
}

type reportInput struct {
	path     string
	bytes    int64
	paths    int
	ops      int
	warmup   int
	ciFrac   float64
	engines  []eng
	samples  [][]observation
	explicit bool
}

func report(w *os.File, in reportInput) {
	_, _ = fmt.Fprintln(w, "OpenAPI engine generation")
	_, _ = fmt.Fprintln(w)
	kind := "generated for this run"
	if in.explicit {
		kind = "file"
	}
	_, _ = fmt.Fprintf(w, "Spec        %s (%s)\n", in.path, kind)
	_, _ = fmt.Fprintf(w, "Size        %d bytes, %d paths, %d operations\n", in.bytes, in.paths, in.ops)
	_, _ = fmt.Fprintf(w, "Generates   server, client, and models, then goimports\n")
	_, _ = fmt.Fprintf(w, "Columns     Load is the engine. Generate is the codegen step, which is the same code for both.\n")
	_, _ = fmt.Fprintf(w, "Go          %s, GOMAXPROCS=%d\n", runtime.Version(), runtime.GOMAXPROCS(0))
	_, _ = fmt.Fprintf(w, "Samples     %d per engine, %d warmup %s discarded\n",
		len(in.samples[0]), in.warmup, plural(in.warmup, "round", "rounds"))
	_, _ = fmt.Fprintf(w, "Method      engines alternate; GC before each sample\n")
	_, _ = fmt.Fprintf(w, "Interval    95%% Student's t interval of the mean (early stop when half-width ≤ %.0f%% of the mean)\n", in.ciFrac*100)
	_, _ = fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "Engine\tLoad\tGenerate\tTotal\tTotal 95% CI\tMedian\tP95\tStdDev\tMin\tMax")
	sums := make([]summary, len(in.engines))
	for i, e := range in.engines {
		load := summarize(column(in.samples[i], func(o observation) time.Duration { return o.load }))
		gen := summarize(column(in.samples[i], func(o observation) time.Duration { return o.generate }))
		total := summarize(totals(in.samples[i]))
		sums[i] = total
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t± %s\t%s\t%s\t%s\t%s\t%s\n",
			e.name,
			formatDuration(load.Mean),
			formatDuration(gen.Mean),
			formatDuration(total.Mean),
			formatDuration(total.CI95Half),
			formatDuration(total.Median),
			formatDuration(total.P95),
			formatDuration(total.StdDev),
			formatDuration(total.Min),
			formatDuration(total.Max),
		)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)

	base, other := sums[0], sums[1]
	writeComparison(w, "Total", in.engines[0].name, in.engines[1].name, base, other)
	writeComparison(w, "Load", in.engines[0].name, in.engines[1].name,
		summarize(column(in.samples[0], func(o observation) time.Duration { return o.load })),
		summarize(column(in.samples[1], func(o observation) time.Duration { return o.load })),
	)

	same := true
	for i := range in.samples {
		first := in.samples[i][0]
		for _, sample := range in.samples[i][1:] {
			if sample.sum != first.sum || sample.nBytes != first.nBytes {
				same = false
			}
		}
		if i > 0 && (first.sum != in.samples[0][0].sum || first.nBytes != in.samples[0][0].nBytes) {
			same = false
		}
	}
	if same {
		_, _ = fmt.Fprintf(w, "Output      %d bytes, identical for every sample of both engines\n", in.samples[0][0].nBytes)
	} else {
		_, _ = fmt.Fprintf(w, "Output      differs: kin %d bytes, libopenapi %d bytes\n", in.samples[0][0].nBytes, in.samples[1][0].nBytes)
	}
}

func writeComparison(w *os.File, label, leftName, rightName string, left, right summary) {
	if left.Mean <= 0 || right.Mean <= 0 {
		return
	}
	ratio := float64(right.Mean) / float64(left.Mean)
	_, _ = fmt.Fprintf(w, "%-12s%s mean is %.2f× %s's\n", label, rightName, ratio, leftName)
	switch {
	case left.N < 2 || right.N < 2:
		_, _ = fmt.Fprintf(w, "%-12sfewer than two samples, so there is no interval\n", "")
	case intervalsOverlap(left, right):
		_, _ = fmt.Fprintf(w, "%-12sthe 95%% intervals overlap, so this run does not separate them\n", "")
	case right.Mean < left.Mean:
		_, _ = fmt.Fprintf(w, "%-12sthe 95%% intervals do not overlap; %s is %.2f× faster\n", "", rightName, 1/ratio)
	default:
		_, _ = fmt.Fprintf(w, "%-12sthe 95%% intervals do not overlap; %s is %.2f× faster\n", "", leftName, ratio)
	}
}

func intervalsOverlap(a, b summary) bool {
	aLow, aHigh := a.Mean-a.CI95Half, a.Mean+a.CI95Half
	bLow, bHigh := b.Mean-b.CI95Half, b.Mean+b.CI95Half
	return aLow <= bHigh && bLow <= aHigh
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := float64(d) / float64(time.Millisecond)
	if ms < 1000 {
		return fmt.Sprintf("%.1f ms", ms)
	}
	return fmt.Sprintf("%.2f s", ms/1000)
}
