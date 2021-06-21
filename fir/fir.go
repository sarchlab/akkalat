package main

import (
	"flag"

	runner "github.com/sarchlab/akkalab/mgpu_config"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/fir"
)

var numData = flag.Int("length", 4096, "The number of samples to filter.")

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := fir.NewBenchmark(runner.Driver())
	benchmark.Length = *numData

	runner.AddBenchmark(benchmark)

	runner.Run()
}
