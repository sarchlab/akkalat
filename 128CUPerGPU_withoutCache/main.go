package main

import (
	"flag"

	"github.com/sarchlab/akkalab/128CUPerGPU_withoutCache/runner"
	"github.com/sarchlab/akkalab/benchmarkselection"
)

var benchmarkFlag = flag.String("benchmark", "fir",
	"Which benchmark to run")

func main() {
	flag.Parse()
	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := benchmarkselection.SelectBenchmark(
		*benchmarkFlag, runner.Driver())

	runner.AddBenchmark(benchmark)
	runner.Run()
}
