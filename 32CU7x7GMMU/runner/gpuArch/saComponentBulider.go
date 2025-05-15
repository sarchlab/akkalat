package gpuArch

import (
	"fmt"
	"log"
	"os"

	"github.com/sarchlab/akita/v4/mem/cache/writearound"
	"github.com/sarchlab/akita/v4/mem/cache/writethrough"
	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm/addresstranslator"
	"github.com/sarchlab/akita/v4/mem/vm/tlb"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cu"
	"github.com/sarchlab/mgpusim/v4/amd/timing/rob"
)

// CU Builder
func (b *shaderArrayBuilder) buildCUs(sa *shaderArray) {
	cuBuilder := cu.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithLog2CachelineSize(b.log2CacheLineSize)

	for i := 0; i < b.numCU; i++ {
		cuName := fmt.Sprintf("%s.CU[%d]", b.name, i)
		computeUnit := cuBuilder.Build(cuName)
		sa.cus = append(sa.cus, computeUnit)

		if b.isaDebugging {
			isaDebug, err := os.Create(
				fmt.Sprintf("isa[%s].debug", cuName))
			if err != nil {
				log.Fatal(err.Error())
			}
			isaDebugger := cu.NewISADebugger(
				log.New(isaDebug, "", 0), computeUnit)

			tracing.CollectTrace(computeUnit, isaDebugger)
		}

		if b.visTracer != nil {
			tracing.CollectTrace(computeUnit, b.visTracer)
		}
	}
}

// Address Translator Builder
func (b *shaderArrayBuilder) buildL1IAddressTranslator(sa *shaderArray) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1IAddrTrans", b.name)
	at := builder.Build(name)
	sa.l1iAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1SAddressTranslator(sa *shaderArray) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1SAddrTrans", b.name)
	at := builder.Build(name)
	sa.l1sAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1VAddressTranslators(sa *shaderArray) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VAddrTrans[%d]", b.name, i)
		at := builder.Build(name)
		sa.l1vATs = append(sa.l1vATs, at)

		if b.visTracer != nil {
			tracing.CollectTrace(at, b.visTracer)
		}
	}
}

// Cache Builder
func (b *shaderArrayBuilder) buildL1VCaches(sa *shaderArray) {
	builder := writearound.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(60).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(4).
		WithNumMSHREntry(16).
		WithTotalByteSize(16 * mem.KB)

	if b.visTracer != nil {
		builder = builder.WithVisTracer(b.visTracer)
	}

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VCache[%d]", b.name, i)
		cache := builder.Build(name)
		sa.l1vCaches = append(sa.l1vCaches, cache)

		if b.memTracer != nil {
			tracing.CollectTrace(cache, b.memTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1SCache(sa *shaderArray) {
	builder := writethrough.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(1).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(4).
		WithNumMSHREntry(16).
		WithTotalByteSize(16 * mem.KB)

	name := fmt.Sprintf("%s.L1SCache", b.name)
	cache := builder.Build(name)
	sa.l1sCache = cache

	if b.visTracer != nil {
		tracing.CollectTrace(cache, b.visTracer)
	}

	if b.memTracer != nil {
		tracing.CollectTrace(cache, b.memTracer)
	}
}

func (b *shaderArrayBuilder) buildL1ICache(sa *shaderArray) {
	builder := writethrough.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(1).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(4).
		WithNumMSHREntry(16).
		WithTotalByteSize(32 * mem.KB).
		WithNumReqsPerCycle(4)

	name := fmt.Sprintf("%s.L1ICache", b.name)
	cache := builder.Build(name)
	sa.l1iCache = cache

	if b.visTracer != nil {
		tracing.CollectTrace(cache, b.visTracer)
	}

	if b.memTracer != nil {
		tracing.CollectTrace(cache, b.memTracer)
	}
}

// TLB Builder
func (b *shaderArrayBuilder) buildL1VTLBs(sa *shaderArray) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(32).
		WithNumReqPerCycle(4)

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VTLB[%d]", b.name, i)
		tlb := builder.Build(name)
		sa.l1vTLBs = append(sa.l1vTLBs, tlb)

		if b.visTracer != nil {
			tracing.CollectTrace(tlb, b.visTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1STLB(sa *shaderArray) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(32).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1STLB", b.name)
	tlb := builder.Build(name)
	sa.l1sTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1ITLB(sa *shaderArray) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(32).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1ITLB", b.name)
	tlb := builder.Build(name)
	sa.l1iTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

// Reorder Buffer Builder
func (b *shaderArrayBuilder) buildL1SReorderBuffer(sa *shaderArray) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1SROB", b.name)
	rob := builder.Build(name)
	sa.l1sROB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1IReorderBuffer(sa *shaderArray) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1IROB", b.name)
	rob := builder.Build(name)
	sa.l1iROB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1VReorderBuffers(sa *shaderArray) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VROB[%d]", b.name, i)
		rob := builder.Build(name)
		sa.l1vROBs = append(sa.l1vROBs, rob)

		if b.visTracer != nil {
			tracing.CollectTrace(rob, b.visTracer)
		}
	}
}
