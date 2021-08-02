package main

import (
	"flag"

	"github.com/sarchlab/akkalat/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/nbody"
)

var numIter = flag.Int("iter", 8, "The number of iterations to run.")
var particles = flag.Int("particles", 1024, "The number of particles in the body.")

func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := nbody.NewBenchmark(runner.Driver())
	benchmark.NumIterations = int32(*numIter)
	benchmark.NumParticles = int32(*particles)

	runner.AddBenchmark(benchmark)

	runner.Run()
}
