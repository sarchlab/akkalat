package runner

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/rob"
)

type shader struct {
	computeUnit *cu.ComputeUnit

	vectorAddressTranslator      *addresstranslator.AddressTranslator
	scalarAddressTranslator      *addresstranslator.AddressTranslator
	instructionAddressTranslator *addresstranslator.AddressTranslator

	vectorRoB      *rob.ReorderBuffer
	instructionRoB *rob.ReorderBuffer
	scalarRoB      *rob.ReorderBuffer

	vectorTLB      *tlb.TLB
	scalarTLB      *tlb.TLB
	instructionTLB *tlb.TLB

	// mem *idealmemcontroller.Comp
}

type shaderBuilder struct {
	gpuID uint64
	name  string

	engine            sim.Engine
	freq              sim.Freq
	log2CacheLineSize uint64
	log2PageSize      uint64
	// memoryLatency     int

	isaDebugging bool
	visTracer    tracing.Tracer
	memTracer    tracing.Tracer

	// globalStorage *mem.Storage
}

func makeShaderBuilder() shaderBuilder {
	builder := shaderBuilder{
		gpuID:             0,
		name:              "Shader",
		freq:              1 * sim.GHz,
		log2CacheLineSize: 6,
		log2PageSize:      12,
	}

	return builder
}

func (b *shaderBuilder) Build(name string) shader {
	b.name = name
	s := shader{}

	b.buildComponents(&s)
	b.connectComponents(&s)

	return s
}

func (b *shaderBuilder) buildComponents(s *shader) {
	b.buildCU(s)

	b.buildVectorAddressTranslator(s)
	b.buildInstructionAddressTranslator(s)
	b.buildScalarAddressTranslator(s)

	b.buildInstructionReorderBuffer(s)
	b.buildScalarReorderBuffer(s)
	b.buildVectorReorderBuffer(s)

	b.buildiTLB(s)
	b.buildvTLB(s)
	b.buildsTLB(s)

	// b.buildMem(s)
}

func (b *shaderBuilder) buildiTLB(s *shader) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1ITLB", b.name)
	tlb := builder.Build(name)
	s.instructionTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *shaderBuilder) buildvTLB(s *shader) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.V1ITLB", b.name)
	tlb := builder.Build(name)
	s.vectorTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *shaderBuilder) buildsTLB(s *shader) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.V1STLB", b.name)
	tlb := builder.Build(name)
	s.scalarTLB = tlb

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *shaderBuilder) buildCU(s *shader) {
	cuBuilder := cu.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithLog2CachelineSize(0)

	cuName := fmt.Sprintf("%s.CU", b.name)
	computeUnit := cuBuilder.Build(cuName)
	s.computeUnit = computeUnit

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

func (b *shaderBuilder) buildVectorAddressTranslator(s *shader) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.VAddrTrans", b.name)
	at := builder.Build(name)
	s.vectorAddressTranslator = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderBuilder) buildInstructionAddressTranslator(s *shader) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.IAddrTrans", b.name)
	at := builder.Build(name)
	s.instructionAddressTranslator = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderBuilder) buildScalarAddressTranslator(s *shader) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.SAddrTrans", b.name)
	at := builder.Build(name)
	s.scalarAddressTranslator = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderBuilder) buildVectorReorderBuffer(s *shader) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.VROB", b.name)
	rob := builder.Build(name)
	s.vectorRoB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderBuilder) buildInstructionReorderBuffer(s *shader) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.IROB", b.name)
	rob := builder.Build(name)
	s.instructionRoB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderBuilder) buildScalarReorderBuffer(s *shader) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.SROB", b.name)
	rob := builder.Build(name)
	s.scalarRoB = rob

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderBuilder) connectComponents(s *shader) {
	b.connectVectorMem(s)
	b.connectScalarMem(s)
	b.connectInstMem(s)
}

func (b *shaderBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	name := fmt.Sprintf("%sto%s", port1.Name(), port2.Name())
	conn := sim.NewDirectConnection(
		name,
		b.engine, b.freq,
	)
	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
}

func (b *shaderBuilder) connectVectorMem(s *shader) {
	cu := s.computeUnit
	rob := s.vectorRoB
	at := s.vectorAddressTranslator
	tlb := s.vectorTLB

	cu.VectorMemModules = &mem.SingleLowModuleFinder{
		LowModule: rob.GetPortByName("Top"),
	}
	b.connectWithDirectConnection(cu.ToVectorMem, rob.GetPortByName("Top"), 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)
}

func (b *shaderBuilder) connectScalarMem(s *shader) {
	cu := s.computeUnit
	rob := s.scalarRoB
	at := s.scalarAddressTranslator
	tlb := s.scalarTLB

	cu.ScalarMem = rob.GetPortByName("Top")
	b.connectWithDirectConnection(rob.GetPortByName("Top"), cu.ToScalarMem, 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)
}

func (b *shaderBuilder) connectInstMem(s *shader) {
	cu := s.computeUnit
	rob := s.instructionRoB
	at := s.instructionAddressTranslator
	tlb := s.instructionTLB

	cu.InstMem = rob.GetPortByName("Top")
	b.connectWithDirectConnection(rob.GetPortByName("Top"), cu.ToInstMem, 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)
}

// func (b *shaderBuilder) buildMem(s *shader) {
// 	memName := fmt.Sprintf("%s.SRAM", b.name)
// 	s.mem = idealmemcontroller.New(memName, b.engine, 1)
// 	s.mem.Latency = b.memoryLatency
// 	s.mem.Storage = b.globalStorage
// }

func (b shaderBuilder) withEngine(e sim.Engine) shaderBuilder {
	b.engine = e
	return b
}

func (b shaderBuilder) withFreq(f sim.Freq) shaderBuilder {
	b.freq = f
	return b
}

func (b shaderBuilder) withGPUID(id uint64) shaderBuilder {
	b.gpuID = id
	return b
}

func (b shaderBuilder) withLog2PageSize(log2Size uint64) shaderBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b shaderBuilder) withIsaDebugging() shaderBuilder {
	b.isaDebugging = true
	return b
}

func (b shaderBuilder) withVisTracer(visTracer tracing.Tracer) shaderBuilder {
	b.visTracer = visTracer
	return b
}

func (b shaderBuilder) withMemTracer(memTracer tracing.Tracer) shaderBuilder {
	b.memTracer = memTracer
	return b
}

func (b shaderBuilder) withLog2CachelineSize(log2Size uint64) shaderBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

// func (b shaderBuilder) withMemoryLatency(latency int) shaderBuilder {
// 	b.memoryLatency = latency
// 	return b
// }

// func (b shaderBuilder) WithGlobalStorage(globalStorage mem.Storage) shaderBuilder {
// 	b.globalStorage = &globalStorage
// 	return b
// }