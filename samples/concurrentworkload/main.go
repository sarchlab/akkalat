package main

import (
	"flag"

	_ "net/http/pprof"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/bitonicsort"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/fir"
)

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	firBenchmark := fir.NewBenchmark(runner.Driver())
	firBenchmark.Length = 10240
	firBenchmark.SelectGPU([]int{1, 2})

	bsBenchmark := bitonicsort.NewBenchmark(runner.Driver())
	bsBenchmark.Length = 64
	bsBenchmark.SelectGPU([]int{3})

	runner.AddBenchmarkWithoutSettingGPUsToUse(firBenchmark)
	runner.AddBenchmarkWithoutSettingGPUsToUse(bsBenchmark)

	runner.Run()
}
