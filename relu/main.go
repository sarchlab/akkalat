package main

import (
	"flag"

	"github.com/sarchlab/akkalab/mgpu_config"
	"gitlab.com/akita/mgpusim/v2/benchmarks/dnn/relu"
)

var numData = flag.Int("length", 4096, "The number of samples to filter.")

func main() {
	flag.Parse()

	runner := new(mgpu_config.Runner).ParseFlag().Init()

	benchmark := relu.NewBenchmark(runner.Driver())
	benchmark.Length = *numData

	runner.AddBenchmark(benchmark)

	runner.Run()
}
