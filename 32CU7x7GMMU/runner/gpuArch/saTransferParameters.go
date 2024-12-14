package gpuArch

import (
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
)

func (b shaderArrayBuilder) WithEngine(e sim.Engine) shaderArrayBuilder {
	b.engine = e
	return b
}

func (b shaderArrayBuilder) WithFreq(f sim.Freq) shaderArrayBuilder {
	b.freq = f
	return b
}

func (b shaderArrayBuilder) WithGPUID(id uint64) shaderArrayBuilder {
	b.gpuID = id
	return b
}

func (b shaderArrayBuilder) WithNumCU(n int) shaderArrayBuilder {
	b.numCU = n
	return b
}

func (b shaderArrayBuilder) WithLog2CachelineSize(
	log2Size uint64,
) shaderArrayBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

func (b shaderArrayBuilder) WithLog2PageSize(
	log2Size uint64,
) shaderArrayBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b shaderArrayBuilder) WithIsaDebugging() shaderArrayBuilder {
	b.isaDebugging = true
	return b
}

func (b shaderArrayBuilder) WithVisTracer(
	visTracer tracing.Tracer,
) shaderArrayBuilder {
	b.visTracer = visTracer
	return b
}

func (b shaderArrayBuilder) WithMemTracer(
	memTracer tracing.Tracer,
) shaderArrayBuilder {
	b.memTracer = memTracer
	return b
}
