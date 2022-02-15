package benchmarkselection

import (
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
	"gitlab.com/akita/mgpusim/v2/driver"
)

func SelectBenchmark(name string, driver *driver.Driver) benchmarks.Benchmark {
	var benchmark benchmarks.Benchmark
	switch name {
	case "aes":
		aes := aes.NewBenchmark(driver)
		aes.Length = 1024
		benchmark = aes
	case "atax":
		atax := atax.NewBenchmark(driver)
		atax.NX = 64
		atax.NY = 64
		benchmark = atax
	case "bicg":
		bicg := bicg.NewBenchmark(driver)
		bicg.NX = 64
		bicg.NY = 64
		benchmark = bicg
	case "bitonicsort":
		bitonicsort := bitonicsort.NewBenchmark(driver)
		bitonicsort.Length = 32
		benchmark = bitonicsort
	case "conv2d":
		conv2d := conv2d.NewBenchmark(driver)
		conv2d.N = 1
		conv2d.C = 3
		conv2d.H = 256
		conv2d.W = 256
		conv2d.KernelChannel = 6
		conv2d.KernelHeight = 3
		conv2d.KernelWidth = 3
		conv2d.PadX = 1
		conv2d.PadY = 1
		conv2d.StrideX = 1
		conv2d.StrideY = 1
		benchmark = conv2d
	case "fastwalshtransform":
		fastwalshtransform := fastwalshtransform.NewBenchmark(driver)
		fastwalshtransform.Length = 64
		benchmark = fastwalshtransform
	case "fir":
		fir := fir.NewBenchmark(driver)
		fir.Length = 1024
		benchmark = fir
	case "fft":
		fft := fft.NewBenchmark(driver)
		fft.Bytes = 2
		fft.Passes = 2
		benchmark = fft
	case "floydwarshall":
		floydwarshall := floydwarshall.NewBenchmark(driver)
		floydwarshall.NumNodes = 64
		floydwarshall.NumIterations = 3
		benchmark = floydwarshall
	case "im2col":
		im2col := im2col.NewBenchmark(driver)
		im2col.N = 1
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
		kmeans := kmeans.NewBenchmark(driver)
		kmeans.NumPoints = 32
		kmeans.NumClusters = 8
		kmeans.NumFeatures = 32
		kmeans.MaxIter = 3
		benchmark = kmeans
	case "matrixmultiplication":
		matrixmultiplication := matrixmultiplication.NewBenchmark(driver)
		matrixmultiplication.X = 64
		matrixmultiplication.Y = 64
		matrixmultiplication.Z = 64
		benchmark = matrixmultiplication
	case "matrixtranspose":
		matrixtranspose := matrixtranspose.NewBenchmark(driver)
		matrixtranspose.Width = 64
		benchmark = matrixtranspose
	case "nbody":
		nbody := nbody.NewBenchmark(driver)
		nbody.NumParticles = 32
		nbody.NumIterations = 3
		benchmark = nbody
	case "nw":
		nw := nw.NewBenchmark(driver)
		nw.SetLength(32)
		benchmark = nw
	case "pagerank":
		pagerank := pagerank.NewBenchmark(driver)
		pagerank.NumNodes = 1024
		pagerank.NumConnections = 1024
		pagerank.MaxIterations = 3
		benchmark = pagerank
	case "relu":
		relu := relu.NewBenchmark(driver)
		relu.Length = 1024
		benchmark = relu
	case "simpleconvolution":
		simpleconvolution := simpleconvolution.NewBenchmark(driver)
		simpleconvolution.Height = 64
		simpleconvolution.Width = 64
		simpleconvolution.SetMaskSize(3)
		benchmark = simpleconvolution
	case "spmv":
		spmv := spmv.NewBenchmark(driver)
		spmv.Dim = 256
		spmv.Sparsity = 0.01
		benchmark = spmv
	case "stencil2d":
		stencil2d := stencil2d.NewBenchmark(driver)
		stencil2d.NumRows = 64
		stencil2d.NumCols = 64
		stencil2d.NumIteration = 3
		benchmark = stencil2d
	default:
		panic("Unknown benchmark")
	}

	return benchmark
}
