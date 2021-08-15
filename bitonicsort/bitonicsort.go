package main

import (
	"flag"

	"github.com/sarchlab/akkalab/mgpu_config"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/bitonicsort"
)

var length = flag.Int("length", 1024, "The length of array to sort.")
var orderAscending = flag.Bool("order-asc", true, "Sorting in ascending order.")

func main() {
	flag.Parse()

	runner := new(mgpu_config.Runner).ParseFlag().Init()

	benchmark := bitonicsort.NewBenchmark(runner.Driver())
	benchmark.Length = *length
	benchmark.OrderAscending = *orderAscending

	runner.AddBenchmark(benchmark)

	runner.Run()
}
