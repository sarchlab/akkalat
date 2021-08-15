package main

import (
	"flag"

	"github.com/sarchlab/akkalab/mgpu_config"
	"gitlab.com/akita/mgpusim/v2/benchmarks/rodinia/nw"
)

var length = flag.Int("length", 64, "The number bases in the gene sequence")

func main() {
	flag.Parse()

	runner := new(mgpu_config.Runner).ParseFlag().Init()

	benchmark := nw.NewBenchmark(runner.Driver())
	benchmark.SetLength(*length)

	runner.AddBenchmark(benchmark)

	runner.Run()
}
