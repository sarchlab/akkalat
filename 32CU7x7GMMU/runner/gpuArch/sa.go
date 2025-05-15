package gpuArch

import (
	"github.com/sarchlab/akita/v4/mem/cache/writearound"
	"github.com/sarchlab/akita/v4/mem/cache/writethrough"
	"github.com/sarchlab/akita/v4/mem/vm/addresstranslator"
	"github.com/sarchlab/akita/v4/mem/vm/tlb"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cu"
	"github.com/sarchlab/mgpusim/v4/amd/timing/rob"
)

type shaderArray struct {
	cus []*cu.ComputeUnit

	l1vROBs []*rob.ReorderBuffer
	l1sROB  *rob.ReorderBuffer
	l1iROB  *rob.ReorderBuffer

	l1vATs []*addresstranslator.Comp
	l1sAT  *addresstranslator.Comp
	l1iAT  *addresstranslator.Comp

	l1vCaches []*writearound.Comp
	l1sCache  *writethrough.Comp
	l1iCache  *writethrough.Comp

	l1vTLBs []*tlb.Comp
	l1sTLB  *tlb.Comp
	l1iTLB  *tlb.Comp
}

type shaderArrayBuilder struct {
	gpuID uint64
	name  string
	numCU int

	engine            sim.Engine
	freq              sim.Freq
	log2CacheLineSize uint64
	log2PageSize      uint64

	isaDebugging bool
	visTracer    tracing.Tracer
	memTracer    tracing.Tracer
}

func makeShaderArrayBuilder() shaderArrayBuilder {
	b := shaderArrayBuilder{
		gpuID:             0,
		name:              "SA",
		numCU:             4,
		freq:              1 * sim.GHz,
		log2CacheLineSize: 6,
		log2PageSize:      12,
	}
	return b
}

func (b shaderArrayBuilder) Build(name string) shaderArray {
	b.name = name
	sa := shaderArray{}

	b.buildComponents(&sa)
	b.connectComponents(&sa)

	return sa
}

func (b *shaderArrayBuilder) buildComponents(sa *shaderArray) {
	b.buildCUs(sa)

	b.buildL1VTLBs(sa)
	b.buildL1VAddressTranslators(sa)
	b.buildL1VReorderBuffers(sa)
	b.buildL1VCaches(sa)

	b.buildL1STLB(sa)
	b.buildL1SAddressTranslator(sa)
	b.buildL1SReorderBuffer(sa)
	b.buildL1SCache(sa)

	b.buildL1ITLB(sa)
	b.buildL1IAddressTranslator(sa)
	b.buildL1IReorderBuffer(sa)
	b.buildL1ICache(sa)
}

func (b *shaderArrayBuilder) connectComponents(sa *shaderArray) {
	b.connectVectorMem(sa)
	b.connectScalarMem(sa)
	b.connectInstMem(sa)
}
