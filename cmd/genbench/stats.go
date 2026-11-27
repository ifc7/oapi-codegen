package main

import (
	"math"
	"slices"
	"time"
)

// summary describes one set of independent samples.
// The 95% interval is a Student's t interval of the mean.
type summary struct {
	N        int
	Mean     time.Duration
	Median   time.Duration
	StdDev   time.Duration
	Min      time.Duration
	Max      time.Duration
	P95      time.Duration
	CI95Half time.Duration
}

func summarize(samples []time.Duration) summary {
	n := len(samples)
	if n == 0 {
		return summary{}
	}
	ordered := slices.Clone(samples)
	slices.Sort(ordered)

	var sum float64
	for _, sample := range ordered {
		sum += float64(sample)
	}
	mean := sum / float64(n)

	var sq float64
	for _, sample := range ordered {
		d := float64(sample) - mean
		sq += d * d
	}
	std := 0.0
	if n > 1 {
		std = math.Sqrt(sq / float64(n-1))
	}

	half := 0.0
	if n > 1 {
		half = studentT95(n-1) * std / math.Sqrt(float64(n))
	}

	return summary{
		N:        n,
		Mean:     time.Duration(mean),
		Median:   median(ordered),
		StdDev:   time.Duration(std),
		Min:      ordered[0],
		Max:      ordered[n-1],
		P95:      percentile(ordered, 0.95),
		CI95Half: time.Duration(half),
	}
}

func median(ordered []time.Duration) time.Duration {
	n := len(ordered)
	if n%2 == 1 {
		return ordered[n/2]
	}
	return (ordered[n/2-1] + ordered[n/2]) / 2
}

// percentile uses the nearest rank: the smallest sample at or above p.
func percentile(ordered []time.Duration, p float64) time.Duration {
	if len(ordered) == 0 {
		return 0
	}
	rank := int(math.Ceil(p*float64(len(ordered)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(ordered) {
		rank = len(ordered) - 1
	}
	return ordered[rank]
}

// studentT95 is the two-tailed 95% critical value for df degrees of freedom.
func studentT95(df int) float64 {
	if df < 1 {
		return math.NaN()
	}
	if df >= len(t95) {
		return 1.96
	}
	return t95[df]
}

// t95[df] for df = 1..30. Larger df uses 1.96.
var t95 = []float64{
	0,
	12.706, 4.303, 3.182, 2.776, 2.571,
	2.447, 2.365, 2.306, 2.262, 2.228,
	2.201, 2.179, 2.160, 2.145, 2.131,
	2.120, 2.110, 2.101, 2.093, 2.086,
	2.080, 2.074, 2.069, 2.064, 2.060,
	2.056, 2.052, 2.048, 2.045, 2.042,
}
