package main

import (
	"flag"

	"github.com/sarchlab/akkalab/model1/runner"
	"gitlab.com/akita/mgpusim/v2/benchmarks"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/bitonicsort"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/fastwalshtransform"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/floydwarshall"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/matrixmultiplication"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/matrixtranspose"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/nbody"
	"gitlab.com/akita/mgpusim/v2/benchmarks/amdappsdk/simpleconvolution"
	"gitlab.com/akita/mgpusim/v2/benchmarks/dnn/conv2d"
	"gitlab.com/akita/mgpusim/v2/benchmarks/dnn/im2col"
	"gitlab.com/akita/mgpusim/v2/benchmarks/dnn/relu"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/aes"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/fir"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/kmeans"
	"gitlab.com/akita/mgpusim/v2/benchmarks/heteromark/pagerank"
	"gitlab.com/akita/mgpusim/v2/benchmarks/polybench/atax"
	"gitlab.com/akita/mgpusim/v2/benchmarks/polybench/bicg"
	"gitlab.com/akita/mgpusim/v2/benchmarks/rodinia/nw"
	"gitlab.com/akita/mgpusim/v2/benchmarks/shoc/fft"
	"gitlab.com/akita/mgpusim/v2/benchmarks/shoc/spmv"
	"gitlab.com/akita/mgpusim/v2/benchmarks/shoc/stencil2d"
)

var benchmarkFlag = flag.String("benchmark", "fir",
	"Which benchmark to run")

func main() {
	flag.Parse()
	runner := new(runner.Runner).ParseFlag().Init()

	var benchmark benchmarks.Benchmark
	switch *benchmarkFlag {
	case "aes":
		aes := aes.NewBenchmark(runner.Driver())
		aes.Length = 104857600
		benchmark = aes
	case "atax":
		atax := atax.NewBenchmark(runner.Driver())
		atax.NX = 8192
		atax.NY = 8192
		benchmark = atax
	case "bicg":
		bicg := bicg.NewBenchmark(runner.Driver())
		bicg.NX = 8192
		bicg.NY = 8192
		benchmark = bicg
	case "bitonicsort":
		bitonicsort := bitonicsort.NewBenchmark(runner.Driver())
		bitonicsort.Length = 65536
		benchmark = bitonicsort
	case "conv2d":
		conv2d := conv2d.NewBenchmark(runner.Driver())
		conv2d.N = 64
		conv2d.C = 3
		conv2d.H = 256
		conv2d.W = 256
		conv2d.KernelChannel = 6
		conv2d.KernelHeight = 3
		conv2d.KernelWidth = 3
		benchmark = conv2d
	case "fastwalshtransform":
		fastwalshtransform := fastwalshtransform.NewBenchmark(runner.Driver())
		fastwalshtransform.Length = 1048576
		benchmark = fastwalshtransform
	case "fir":
		fir := fir.NewBenchmark(runner.Driver())
		fir.Length = 10485760
		benchmark = fir
	case "fft":
		fft := fft.NewBenchmark(runner.Driver())
		fft.Bytes = 128
		fft.Passes = 2
		benchmark = fft
	case "floydwarshall":
		floydwarshall := floydwarshall.NewBenchmark(runner.Driver())
		floydwarshall.NumNodes = 8192
		floydwarshall.NumIterations = 3
		benchmark = floydwarshall
	case "im2col":
		im2col := im2col.NewBenchmark(runner.Driver())
		im2col.N = 64
		im2col.C = 3
		im2col.H = 256
		im2col.W = 256
		im2col.KernelHeight = 3
		im2col.KernelWidth = 3
		im2col.PadX = 0
		im2col.PadY = 0
		im2col.StrideX = 1
		im2col.StrideY = 1
		im2col.DilateX = 1
		im2col.DilateY = 1
		benchmark = im2col
	case "kmeans":
		kmeans := kmeans.NewBenchmark(runner.Driver())
		kmeans.NumPoints = 8192
		kmeans.NumClusters = 8
		kmeans.NumFeatures = 32
		kmeans.MaxIter = 3
		benchmark = kmeans
	case "matrixmultiplication":
		matrixmultiplication := matrixmultiplication.NewBenchmark(runner.Driver())
		matrixmultiplication.X = 8192
		matrixmultiplication.Y = 8192
		matrixmultiplication.Z = 8192
		benchmark = matrixmultiplication
	case "matrixtranspose":
		matrixtranspose := matrixtranspose.NewBenchmark(runner.Driver())
		matrixtranspose.Width = 8192
		benchmark = matrixtranspose
	case "nbody":
		nbody := nbody.NewBenchmark(runner.Driver())
		nbody.NumParticles = 65536
		nbody.NumIterations = 3
		benchmark = nbody
	case "nw":
		nw := nw.NewBenchmark(runner.Driver())
		nw.SetLength(8192)
		benchmark = nw
	case "pagerank":
		pagerank := pagerank.NewBenchmark(runner.Driver())
		pagerank.NumNodes = 104857600
		pagerank.NumConnections = 104857600
		pagerank.NumIterations = 3
		benchmark = pagerank
	case "relu":
		relu := relu.NewBenchmark(runner.Driver())
		relu.Length = 104857600
		benchmark = relu
	case "simpleconvolution":
		simpleconvolution := simpleconvolution.NewBenchmark(runner.Driver())
		simpleconvolution.Height = 4096
		simpleconvolution.Width = 4096
		simpleconvolution.SetMaskSize(3)
		benchmark = simpleconvolution
	case "spmv":
		spmv := spmv.NewBenchmark(runner.Driver())
		spmv.Dim = 65536
		spmv.Sparsity = 0.001
		benchmark = spmv
	case "stencil2d":
		stencil2d := stencil2d.NewBenchmark(runner.Driver())
		stencil2d.Height = 8192
		stencil2d.Width = 8192
		stencil2d.NumIteration = 3
		benchmark = stencil2d
	default:
		panic("Unknown benchmark")
	}

	runner.AddBenchmark(benchmark)
	runner.Run()
}
