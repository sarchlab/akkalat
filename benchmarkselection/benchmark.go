package benchmarkselection

import (
	"github.com/sarchlab/mgpusim/v3/benchmarks"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/bitonicsort"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/fastwalshtransform"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/floydwarshall"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/matrixmultiplication"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/matrixtranspose"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/nbody"
	"github.com/sarchlab/mgpusim/v3/benchmarks/amdappsdk/simpleconvolution"
	"github.com/sarchlab/mgpusim/v3/benchmarks/dnn/conv2d"
	"github.com/sarchlab/mgpusim/v3/benchmarks/dnn/im2col"
	"github.com/sarchlab/mgpusim/v3/benchmarks/dnn/relu"
	"github.com/sarchlab/mgpusim/v3/benchmarks/heteromark/aes"
	"github.com/sarchlab/mgpusim/v3/benchmarks/heteromark/fir"
	"github.com/sarchlab/mgpusim/v3/benchmarks/heteromark/kmeans"
	"github.com/sarchlab/mgpusim/v3/benchmarks/heteromark/pagerank"
	"github.com/sarchlab/mgpusim/v3/benchmarks/polybench/atax"
	"github.com/sarchlab/mgpusim/v3/benchmarks/polybench/bicg"
	"github.com/sarchlab/mgpusim/v3/benchmarks/rodinia/nw"
	"github.com/sarchlab/mgpusim/v3/benchmarks/shoc/fft"
	"github.com/sarchlab/mgpusim/v3/benchmarks/shoc/spmv"
	"github.com/sarchlab/mgpusim/v3/benchmarks/shoc/stencil2d"
	"github.com/sarchlab/mgpusim/v3/driver"
)

func SelectBenchmark(name string, driver *driver.Driver) benchmarks.Benchmark {
	var benchmark benchmarks.Benchmark
	switch name {
	case "aes":
		aes := aes.NewBenchmark(driver)
		aes.Length = 10485760 * 5
		benchmark = aes
	case "atax":
		atax := atax.NewBenchmark(driver)
		atax.NX = 4096
		atax.NY = 4096
		benchmark = atax
	case "bicg":
		bicg := bicg.NewBenchmark(driver)
		bicg.NX = 4096
		bicg.NY = 4096
		benchmark = bicg
	case "bitonicsort":
		bitonicsort := bitonicsort.NewBenchmark(driver)
		bitonicsort.Length = 655360
		benchmark = bitonicsort
	case "conv2d":
		conv2d := conv2d.NewBenchmark(driver)
		conv2d.N = 8
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
		fastwalshtransform.Length = 1048576
		benchmark = fastwalshtransform
	case "fir":
		fir := fir.NewBenchmark(driver)
		// fir.Length = 10485760
		fir.Length = 64
		benchmark = fir
	case "fft":
		fft := fft.NewBenchmark(driver)
		fft.Bytes = 128
		fft.Passes = 2
		benchmark = fft
	case "floydwarshall":
		floydwarshall := floydwarshall.NewBenchmark(driver)
		floydwarshall.NumNodes = 1024
		floydwarshall.NumIterations = 1024
		benchmark = floydwarshall
	case "im2col":
		im2col := im2col.NewBenchmark(driver)
		im2col.N = 16
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
		kmeans.NumPoints = 1048576
		kmeans.NumClusters = 8
		kmeans.NumFeatures = 32
		kmeans.MaxIter = 3
		benchmark = kmeans
	case "matrixmultiplication":
		matrixmultiplication := matrixmultiplication.NewBenchmark(driver)
		matrixmultiplication.X = 2048
		matrixmultiplication.Y = 2048
		matrixmultiplication.Z = 2048
		benchmark = matrixmultiplication
	case "matrixtranspose":
		matrixtranspose := matrixtranspose.NewBenchmark(driver)
		matrixtranspose.Width = 4096
		benchmark = matrixtranspose
	case "nbody":
		nbody := nbody.NewBenchmark(driver)
		nbody.NumParticles = 104857600
		nbody.NumIterations = 1024
		benchmark = nbody
	case "nw":
		nw := nw.NewBenchmark(driver)
		nw.SetLength(8192)
		benchmark = nw
	case "pagerank":
		pagerank := pagerank.NewBenchmark(driver)
		pagerank.NumNodes = 262144
		pagerank.NumConnections = 1048576
		pagerank.MaxIterations = 3
		benchmark = pagerank
	case "relu":
		relu := relu.NewBenchmark(driver)
		relu.Length = 10485760
		benchmark = relu
	case "simpleconvolution":
		simpleconvolution := simpleconvolution.NewBenchmark(driver)
		simpleconvolution.Height = 4096
		simpleconvolution.Width = 4096
		simpleconvolution.SetMaskSize(3)
		benchmark = simpleconvolution
	case "spmv":
		spmv := spmv.NewBenchmark(driver)
		spmv.Dim = 10485760
		spmv.Sparsity = 0.000000001
		benchmark = spmv
	case "stencil2d":
		stencil2d := stencil2d.NewBenchmark(driver)
		stencil2d.NumRows = 4096
		stencil2d.NumCols = 4096
		stencil2d.NumIteration = 3
		benchmark = stencil2d
	default:
		panic("Unknown benchmark")
	}

	return benchmark
}
