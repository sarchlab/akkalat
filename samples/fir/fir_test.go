package main_test

import (
	"testing"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/fir"
)

func BenchmarkFIR(t *testing.B) {
	runner := runner.Runner{}
	runner.Timing = true
	runner.Verify = true
	runner.Parallel = true
	runner.GPUIDs = []int{1}
	runner.Init()

	benchmark := fir.NewBenchmark(runner.Driver())
	benchmark.Length = 4096

	runner.AddBenchmark(benchmark)

	runner.Run()
}
