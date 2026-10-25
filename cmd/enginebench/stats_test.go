package main

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
)

func TestSummarizeOdd(t *testing.T) {
	samples := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}
	got := summarize(samples)
	if got.N != 5 || got.Mean != 3*time.Millisecond || got.Median != 3*time.Millisecond {
		t.Fatalf("mean/median: %+v", got)
	}
	if got.Min != time.Millisecond || got.Max != 5*time.Millisecond || got.P95 != 5*time.Millisecond {
		t.Fatalf("spread: %+v", got)
	}
	// Sample standard deviation of 1..5 ms is sqrt(2.5) ms.
	wantStd := time.Duration(math.Sqrt(2.5) * float64(time.Millisecond))
	if math.Abs(float64(got.StdDev-wantStd)) > float64(time.Microsecond) {
		t.Fatalf("stddev %s, want %s", got.StdDev, wantStd)
	}
	if got.CI95Half <= 0 {
		t.Fatal("expected a positive confidence interval")
	}
}

func TestSummarizeEvenMedian(t *testing.T) {
	samples := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
	}
	got := summarize(samples)
	if got.Median != 2500*time.Microsecond {
		t.Fatalf("median %s", got.Median)
	}
}

func TestSpecDeclaresPathParameters(t *testing.T) {
	path, err := writeSpec(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := kin.Engine{}.Load(path, engine.LoadOptions{})
	if err != nil {
		body, _ := os.ReadFile(path)
		t.Fatalf("load: %v\n%s", err, body)
	}
	item := doc.Paths.Value("/groups/0/items/{id}")
	if item == nil || item.Get == nil {
		t.Fatalf("missing get operation: %#v", item)
	}
	if len(item.Get.Parameters) != 1 || item.Get.Parameters[0].Value == nil || item.Get.Parameters[0].Value.In != "path" {
		body, _ := os.ReadFile(path)
		t.Fatalf("parameters: %#v\n%s", item.Get.Parameters, body)
	}
	if _, err := codegen.Generate(doc, codegen.Configuration{
		PackageName: "api",
		Generate:    codegen.GenerateOptions{Server: true, Client: true, Models: true},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPercentile(t *testing.T) {
	ordered := []time.Duration{10, 20, 30, 40}
	if percentile(ordered, 0.50) != 20 {
		t.Fatalf("p50 %s", percentile(ordered, 0.50))
	}
	if percentile(ordered, 1) != 40 {
		t.Fatalf("p100 %s", percentile(ordered, 1))
	}
}
