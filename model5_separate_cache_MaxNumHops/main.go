package main

import (
	"flag"

	"github.com/sarchlab/akkalab/benchmarkselection"
	"github.com/sarchlab/akkalab/model5_separate_cache_MaxNumHops/runner"
	// "github.com/sarchlab/akkalab/model1_separate_cache/runner"
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
