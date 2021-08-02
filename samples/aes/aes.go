package main

import (
	"flag"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/aes"
)

var lenInput = flag.Int("length", 65536, "The length of array to sort.")

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := aes.NewBenchmark(runner.Driver())
	benchmark.Length = *lenInput

	runner.AddBenchmark(benchmark)

	runner.Run()
}
