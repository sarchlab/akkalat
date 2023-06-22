package runner

import (
	"fmt"
	"log"
	"os"

	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm/addresstranslator"
	"github.com/sarchlab/akita/v3/mem/vm/tlb"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/timing/cu"
	"github.com/sarchlab/mgpusim/v3/timing/rob"
)

type shader struct {
	computeUnits []*cu.ComputeUnit

	vectorAddressTranslators     []*addresstranslator.AddressTranslator
	scalarAddressTranslator      *addresstranslator.AddressTranslator
	instructionAddressTranslator *addresstranslator.AddressTranslator

	vectorRoBs     []*rob.ReorderBuffer
	instructionRoB *rob.ReorderBuffer
	scalarRoB      *rob.ReorderBuffer

	vectorTLBs     []*tlb.TLB
	scalarTLB      *tlb.TLB
	instructionTLB *tlb.TLB

	// mem *idealmemcontroller.Comp
}

type shaderBuilder struct {
	gpuID uint64
	name  string

	numOfCU int

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
		numOfCU:           4,
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

	for i := 0; i < b.numOfCU; i++ {
		name := fmt.Sprintf("%s.V1ITLB[%v]", b.name, i)
		tlb := builder.Build(name)
		s.vectorTLBs = append(s.vectorTLBs, tlb)

		if b.visTracer != nil {
			tracing.CollectTrace(tlb, b.visTracer)
		}
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
		WithLog2CachelineSize(6)

	for i := 0; i < b.numOfCU; i++ {
		cuName := fmt.Sprintf("%s.CU[%v]", b.name, i)
		computeUnit := cuBuilder.Build(cuName)
		s.computeUnits = append(s.computeUnits, computeUnit)

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

func (b *shaderBuilder) buildVectorAddressTranslator(s *shader) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	for i := 0; i < b.numOfCU; i++ {
		name := fmt.Sprintf("%s.VAddrTrans[%v]", b.name, i)
		at := builder.Build(name)
		s.vectorAddressTranslators = append(s.vectorAddressTranslators, at)

		if b.visTracer != nil {
			tracing.CollectTrace(at, b.visTracer)
		}
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

	for i := 0; i < b.numOfCU; i++ {
		name := fmt.Sprintf("%s.VROB", b.name)
		rob := builder.Build(name)
		s.vectorRoBs = append(s.vectorRoBs, rob)

		if b.visTracer != nil {
			tracing.CollectTrace(rob, b.visTracer)
		}
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
	for i := 0; i < b.numOfCU; i++ {
		cu := s.computeUnits[i]
		rob := s.vectorRoBs[i]
		at := s.vectorAddressTranslators[i]
		tlb := s.vectorTLBs[i]

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

}

func (b *shaderBuilder) connectScalarMem(s *shader) {
	rob := s.scalarRoB
	at := s.scalarAddressTranslator
	tlb := s.scalarTLB

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	robTopPort := rob.GetPortByName("Top")
	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(robTopPort, 8)
	for i := 0; i < b.numOfCU; i++ {
		cu := s.computeUnits[i]
		cu.ScalarMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToScalarMem, 8)
	}

}

func (b *shaderBuilder) connectInstMem(s *shader) {
	rob := s.instructionRoB
	at := s.instructionAddressTranslator
	tlb := s.instructionTLB

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(at.GetPortByName("Translation"), tlbTopPort, 8)

	robTopPort := rob.GetPortByName("Top")
	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(robTopPort, 8)
	for i := 0; i < b.numOfCU; i++ {
		cu := s.computeUnits[i]
		cu.InstMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToInstMem, 8)
	}
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

func (b shaderBuilder) withNumOfCU(number int) shaderBuilder {
	b.numOfCU = number
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
