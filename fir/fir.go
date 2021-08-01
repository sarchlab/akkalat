package main

import (
	"flag"

	"gitlab.com/akita/mgpusim/benchmarks/heteromark/fir"
	"honnef.co/go/tools/lintcmd/runner"
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
