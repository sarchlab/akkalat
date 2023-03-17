package runner

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/cache/writethrough"
	"gitlab.com/akita/mem/v3/idealmemcontroller"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/emu"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/rob"
)

var numTilesBuild int

type tile struct {
	cu *cu.ComputeUnit

	l1vROB *rob.ReorderBuffer
	l1sROB *rob.ReorderBuffer
	l1iROB *rob.ReorderBuffer

	l1vAT *addresstranslator.AddressTranslator
	l1sAT *addresstranslator.AddressTranslator
	l1iAT *addresstranslator.AddressTranslator

	l1vTLB *tlb.TLB
	l1sTLB *tlb.TLB
	l1iTLB *tlb.TLB

	l1sCache *writethrough.Cache
	l1iCache *writethrough.Cache

	mem *idealmemcontroller.Comp
}

type tileBuilder struct {
	gpuID uint64
	name  string

	engine            sim.Engine
	freq              sim.Freq
	log2CacheLineSize uint64
	log2PageSize      uint64
	memLatency        int

	isaDebugging  bool
	decoder       emu.Decoder
	visTracer     tracing.Tracer
	memTracer     tracing.Tracer
	globalStorage *mem.Storage

	connectionCount int
}

func makeTileBuilder() tileBuilder {
	b := tileBuilder{
		gpuID:             0,
		name:              "Tile",
		freq:              1 * sim.GHz,
		log2CacheLineSize: 6,
		log2PageSize:      12,
	}
	return b
}

func (b tileBuilder) withEngine(e sim.Engine) tileBuilder {
	b.engine = e
	return b
}

func (b tileBuilder) withFreq(f sim.Freq) tileBuilder {
	b.freq = f
	return b
}

func (b tileBuilder) withGPUID(id uint64) tileBuilder {
	b.gpuID = id
	return b
}

func (b tileBuilder) withLog2CachelineSize(
	log2Size uint64,
) tileBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

func (b tileBuilder) withLog2PageSize(
	log2Size uint64,
) tileBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b tileBuilder) withIsaDebugging() tileBuilder {
	b.isaDebugging = true
	return b
}

func (b tileBuilder) withVisTracer(
	visTracer tracing.Tracer,
) tileBuilder {
	b.visTracer = visTracer
	return b
}

func (b tileBuilder) withMemTracer(
	memTracer tracing.Tracer,
) tileBuilder {
	b.memTracer = memTracer
	return b
}

func (b tileBuilder) withDecoder(
	d emu.Decoder,
) tileBuilder {
	b.decoder = d
	return b
}

func (b tileBuilder) WithGlobalStorage(storage *mem.Storage) tileBuilder {
	b.globalStorage = storage
	return b
}

func (b tileBuilder) Build(name string) tile {
	b.name = name
	t := tile{}

	b.buildComponents(&t)
	b.connectComponents(&t)

	numTilesBuild++
	// fmt.Printf("Built tile %d\n", numTilesBuild)

	return t
}

func (b *tileBuilder) buildComponents(t *tile) {
	b.buildCU(t)
	b.buildMemory(t)

	b.buildL1VTLBs(t)
	b.buildL1VAddressTranslators(t)
	b.buildL1VReorderBuffers(t)

	b.buildL1STLB(t)
	b.buildL1SAddressTranslator(t)
	b.buildL1SReorderBuffer(t)
	b.buildL1SCache(t)

	b.buildL1ITLB(t)
	b.buildL1IAddressTranslator(t)
	b.buildL1IReorderBuffer(t)
	b.buildL1ICache(t)
}

func (b *tileBuilder) connectComponents(sa *tile) {
	b.connectVectorMem(sa)
	b.connectScalarMem(sa)
	b.connectInstMem(sa)
}

func (b *tileBuilder) connectVectorMem(t *tile) {
	cu := t.cu
	rob := t.l1vROB
	at := t.l1vAT
	tlb := t.l1vTLB

	cu.VectorMemModules = &mem.SingleLowModuleFinder{
		LowModule: rob.GetPortByName("Top"),
	}
	b.connectWithDirectConnection(cu.ToVectorMem,
		rob.GetPortByName("Top"), 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(
		rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)
}

func (b *tileBuilder) connectScalarMem(t *tile) {
	cu := t.cu
	rob := t.l1sROB
	at := t.l1sAT
	tlb := t.l1sTLB
	l1s := t.l1sCache

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	at.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: l1s.GetPortByName("Top"),
	})
	b.connectWithDirectConnection(
		l1s.GetPortByName("Top"), at.GetPortByName("Bottom"), 8)

	b.connectWithDirectConnection(rob.GetPortByName("Top"), cu.ToScalarMem, 8)
	cu.ScalarMem = rob.GetPortByName("Top")
}

func (b *tileBuilder) connectInstMem(t *tile) {
	cu := t.cu
	rob := t.l1iROB
	at := t.l1iAT
	tlb := t.l1iTLB
	l1i := t.l1iCache

	l1iTopPort := l1i.GetPortByName("Top")
	rob.BottomUnit = l1iTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), l1iTopPort, 8)

	atTopPort := at.GetPortByName("Top")
	l1i.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: atTopPort,
	})
	b.connectWithDirectConnection(l1i.GetPortByName("Bottom"), atTopPort, 8)

	// rob.BottomUnit = atTopPort
	// b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	b.connectWithDirectConnection(rob.GetPortByName("Top"), cu.ToInstMem, 8)
	cu.InstMem = rob.GetPortByName("Top")
}

func (b *tileBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	name := fmt.Sprintf("%s.Conn[%d]", b.name, b.connectionCount)
	b.connectionCount++

	conn := sim.NewDirectConnection(
		name,
		b.engine, b.freq,
	)

	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
}

func (b *tileBuilder) buildCU(t *tile) {
	cuBuilder := cu.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		// WithSIMDCount(1).
		// WithVGPRCount([]int{16384}).
		// WithSGPRCount(3200).
		WithLog2CachelineSize(b.log2CacheLineSize)

	cuName := fmt.Sprintf("%s.CU", b.name)
	computeUnit := cuBuilder.Build(cuName)
	t.cu = computeUnit

	if b.isaDebugging {
		isaDebug, err := os.Create(
			fmt.Sprintf("isa_%s.debug", cuName))
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

func (b *tileBuilder) buildMemory(t *tile) {
	memName := fmt.Sprintf("%s.SRAM", b.name)
	t.mem = idealmemcontroller.New(memName, b.engine, 1)
	t.mem.Latency = b.memLatency
	t.mem.Storage = b.globalStorage

	if b.visTracer != nil {
		tracing.CollectTrace(t.mem, b.visTracer)
	}
}

func (b *tileBuilder) buildL1VReorderBuffers(t *tile) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1VROB", b.name)
	rob := builder.Build(name)
	t.l1vROB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *tileBuilder) buildL1VAddressTranslators(t *tile) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1VAddrTrans", b.name)
	at := builder.Build(name)
	t.l1vAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *tileBuilder) buildL1VTLBs(t *tile) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1VTLB", b.name)
	tlb := builder.Build(name)
	t.l1vTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *tileBuilder) buildL1SReorderBuffer(t *tile) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1SROB", b.name)
	rob := builder.Build(name)
	t.l1sROB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *tileBuilder) buildL1SAddressTranslator(t *tile) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1SAddrTrans", b.name)
	at := builder.Build(name)
	t.l1sAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *tileBuilder) buildL1STLB(t *tile) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(4).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1STLB", b.name)
	tlb := builder.Build(name)
	t.l1sTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *tileBuilder) buildL1IReorderBuffer(t *tile) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1IROB", b.name)
	rob := builder.Build(name)
	t.l1iROB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *tileBuilder) buildL1IAddressTranslator(t *tile) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1IAddrTrans", b.name)
	at := builder.Build(name)
	t.l1iAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *tileBuilder) buildL1ITLB(t *tile) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(4).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1ITLB", b.name)
	tlb := builder.Build(name)
	t.l1iTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *tileBuilder) buildL1SCache(t *tile) {
	builder := writethrough.NewBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(1).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(4).
		WithNumMSHREntry(16).
		WithTotalByteSize(1 * mem.KB)

	name := fmt.Sprintf("%s.L1SCache", b.name)
	cache := builder.Build(name)
	t.l1sCache = cache

	if b.visTracer != nil {
		tracing.CollectTrace(cache, b.visTracer)
	}

	if b.memTracer != nil {
		tracing.CollectTrace(cache, b.memTracer)
	}
}

func (b *tileBuilder) buildL1ICache(t *tile) {
	builder := writethrough.NewBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(1).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(4).
		WithNumMSHREntry(16).
		WithTotalByteSize(1 * mem.KB). // could be divided by 4
		WithNumReqsPerCycle(4)

	name := fmt.Sprintf("%s.L1ICache", b.name)
	cache := builder.Build(name)
	t.l1iCache = cache

	if b.visTracer != nil {
		tracing.CollectTrace(cache, b.visTracer)
	}

	if b.memTracer != nil {
		tracing.CollectTrace(cache, b.memTracer)
	}
}
