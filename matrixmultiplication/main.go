package main

import (
	"flag"

	_ "net/http/pprof"

	"github.com/sarchlab/akkalab/mgpu_config"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/matrixmultiplication"
)

var xFlag = flag.Uint("x", 64, "The height of the first matrix.")
var yFlag = flag.Uint("y", 64, "The width of the first matrix and the height of the second matrix.")
var zFlag = flag.Uint("z", 64, "The width of the second matrix.")

func main() {
	flag.Parse()

	runner := new(mgpu_config.Runner).ParseFlag().Init()

	benchmark := matrixmultiplication.NewBenchmark(runner.Driver())
	benchmark.X = uint32(*xFlag)
	benchmark.Y = uint32(*yFlag)
	benchmark.Z = uint32(*zFlag)

	runner.AddBenchmark(benchmark)

	runner.Run()
}
