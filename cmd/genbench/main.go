// Command genbench times code generation on one OpenAPI document: loading it
// with libopenapi, then generating server, client and models from it.
//
// The default document is written to a temp file: many paths and schemas
// that reference each other, large enough that loading and generating
// dominate the timer. -spec times a file, or a document fetched from a URL,
// instead.
//
// The garbage collector runs before each sample so one sample does not pay
// for the previous one's garbage. Warmup rounds are discarded. The run stops
// once the 95% confidence interval is within -ci of the mean, and always
// reports that interval.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"text/tabwriter"
	"time"

	"github.com/ifc7/oapi-codegen/v2/pkg/codegen"
	"github.com/ifc7/oapi-codegen/v2/pkg/engine"
	"github.com/ifc7/oapi-codegen/v2/pkg/engine/libopenapi"
	"github.com/ifc7/oapi-codegen/v2/pkg/model"
)

func main() {
	specPath := flag.String("spec", "", "OpenAPI file or http(s) URL to time; empty writes a generated spec")
	resources := flag.Int("resources", 80, "paths in the generated spec, ignored when -spec is set")
	warmupN := flag.Int("warmup", 2, "unmeasured rounds")
	samplesN := flag.Int("samples", 0, "samples; 0 stops once the confidence interval is tight")
	minSamples := flag.Int("min-samples", 12, "minimum samples when -samples is 0")
	maxSamples := flag.Int("max-samples", 30, "maximum samples when -samples is 0")
	ciFrac := flag.Float64("ci", 0.05, "stop when the 95% CI half-width is within this fraction of the mean")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile of the measured samples to this file")
	memProfile := flag.String("memprofile", "", "write an allocation profile (alloc_space) after the measured samples to this file")
	skipFmt := flag.Bool("skip-fmt", false, "skip formatting the output, to time the rest of generation on its own")
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

	cfg := codegen.Configuration{
		PackageName: "api",
		Generate: codegen.GenerateOptions{
			Server: true,
			Client: true,
			Models: true,
		},
		OutputOptions: codegen.OutputOptions{SkipFmt: *skipFmt},
	}

	_, _ = fmt.Fprintf(os.Stderr, "warming up (%d %s)...\n", *warmupN, plural(*warmupN, "round", "rounds"))
	for round := 0; round < *warmupN; round++ {
		if _, err := measure(path, cfg); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	target := *samplesN
	if target == 0 {
		target = *maxSamples
	}
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cpuprofile: %v\n", err)
			os.Exit(1)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cpuprofile: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		}()
	}
	var taken []observation
	var docPaths, docOps int
	if *samplesN == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "measuring up to %d samples\n", target)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "measuring %d samples\n", target)
	}
	for n := 0; n < target; n++ {
		obs, err := measure(path, cfg)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		if n == 0 {
			docPaths, docOps = count(obs.doc)
		}
		obs.doc = nil
		taken = append(taken, obs)
		_, _ = fmt.Fprintf(os.Stderr, "  sample %d  %s\n", n+1, formatDuration(obs.total))
		if *samplesN == 0 && n+1 >= *minSamples && intervalTight(taken, *ciFrac) {
			break
		}
	}

	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "memprofile: %v\n", err)
			os.Exit(1)
		}
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "memprofile: %v\n", err)
			os.Exit(1)
		}
		_ = f.Close()
	}

	report(os.Stdout, reportInput{
		path:     path,
		bytes:    info.Size(),
		paths:    docPaths,
		ops:      docOps,
		warmup:   *warmupN,
		ciFrac:   *ciFrac,
		samples:  taken,
		explicit: *specPath != "",
	})
}

type observation struct {
	load     time.Duration
	generate time.Duration
	total    time.Duration
	nBytes   int
	// mallocs and allocBytes are the heap allocations of the whole sample,
	// load and generate. They barely vary between samples, so they show a
	// change that the timing noise would hide.
	mallocs    uint64
	allocBytes uint64
	sum        [sha256.Size]byte
	doc        *model.Document
}

func measure(path string, cfg codegen.Configuration) (observation, error) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	doc, err := libopenapi.Engine{}.Load(path, engine.LoadOptions{})
	if err != nil {
		return observation{}, fmt.Errorf("load: %w", err)
	}
	loaded := time.Since(started)
	code, err := codegen.Generate(doc, cfg)
	if err != nil {
		return observation{}, fmt.Errorf("generate: %w", err)
	}
	total := time.Since(started)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	return observation{
		mallocs:    after.Mallocs - before.Mallocs,
		allocBytes: after.TotalAlloc - before.TotalAlloc,
		load:       loaded,
		generate:   total - loaded,
		total:      total,
		nBytes:     len(code),
		sum:        sha256.Sum256([]byte(code)),
		doc:        doc,
	}, nil
}

func openSpec(path string, resources int) (string, func(), error) {
	if path != "" && !isURL(path) {
		return path, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "genbench")
	if err != nil {
		return "", nil, err
	}
	if path != "" {
		downloaded, err := download(dir, path)
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, err
		}
		return downloaded, func() { _ = os.RemoveAll(dir) }, nil
	}
	written, err := writeSpec(dir, resources)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return written, func() { _ = os.RemoveAll(dir) }, nil
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

func intervalTight(samples []observation, frac float64) bool {
	s := summarize(totals(samples))
	return s.Mean != 0 && float64(s.CI95Half) <= frac*float64(s.Mean)
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
	samples  []observation
	explicit bool
}

func report(w *os.File, in reportInput) {
	_, _ = fmt.Fprintln(w, "Code generation")
	_, _ = fmt.Fprintln(w)
	kind := "generated for this run"
	if in.explicit {
		kind = "file"
	}
	_, _ = fmt.Fprintf(w, "Spec        %s (%s)\n", in.path, kind)
	_, _ = fmt.Fprintf(w, "Size        %d bytes, %d paths, %d operations\n", in.bytes, in.paths, in.ops)
	_, _ = fmt.Fprintf(w, "Generates   server, client, and models, then goimports\n")
	_, _ = fmt.Fprintf(w, "Columns     Load reads the spec with libopenapi. Generate is the codegen step.\n")
	_, _ = fmt.Fprintf(w, "Go          %s, GOMAXPROCS=%d\n", runtime.Version(), runtime.GOMAXPROCS(0))
	_, _ = fmt.Fprintf(w, "Samples     %d, %d warmup %s discarded\n", len(in.samples), in.warmup, plural(in.warmup, "round", "rounds"))
	_, _ = fmt.Fprintf(w, "Method      GC before each sample\n")
	_, _ = fmt.Fprintf(w, "Interval    95%% Student's t interval of the mean (early stop when half-width ≤ %.0f%% of the mean)\n", in.ciFrac*100)
	_, _ = fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "Load\tGenerate\tTotal\tTotal 95% CI\tMedian\tP95\tStdDev\tMin\tMax")
	load := summarize(column(in.samples, func(o observation) time.Duration { return o.load }))
	gen := summarize(column(in.samples, func(o observation) time.Duration { return o.generate }))
	total := summarize(totals(in.samples))
	_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t± %s\t%s\t%s\t%s\t%s\t%s\n",
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
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)

	mallocs := make([]float64, len(in.samples))
	allocMB := make([]float64, len(in.samples))
	for i, sample := range in.samples {
		mallocs[i] = float64(sample.mallocs)
		allocMB[i] = float64(sample.allocBytes) / (1 << 20)
	}
	_, _ = fmt.Fprintf(w, "Allocations %.2f million allocs, %.0f MB per run (mean of %d; load and generate)\n",
		mean(mallocs)/1e6, mean(allocMB), len(in.samples))

	first := in.samples[0]
	same := true
	for _, sample := range in.samples[1:] {
		if sample.sum != first.sum || sample.nBytes != first.nBytes {
			same = false
		}
	}
	if same {
		_, _ = fmt.Fprintf(w, "Output      %d bytes, identical for every sample\n", first.nBytes)
	} else {
		_, _ = fmt.Fprintf(w, "Output      differs between samples, which should not happen\n")
	}
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

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
