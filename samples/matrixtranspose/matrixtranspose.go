package main

import (
	"flag"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/matrixtranspose"
)

var dataWidth = flag.Int("width", 256, "The dimension of the square matrix.")

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := matrixtranspose.NewBenchmark(runner.Driver())
	benchmark.Width = *dataWidth

	runner.AddBenchmark(benchmark)

	runner.Run()
}
