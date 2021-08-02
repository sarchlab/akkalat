package main

import (
	"flag"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/fastwalshtransform"
)

var length = flag.Int("length", 1024, "The length of the array that will be transformed")

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := fastwalshtransform.NewBenchmark(runner.Driver())
	benchmark.Length = uint32(*length)

	runner.AddBenchmark(benchmark)

	runner.Run()
}
