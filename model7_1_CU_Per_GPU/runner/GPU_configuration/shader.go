package runner

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
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
}

type shaderBuilder struct {
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
}

func (b *shaderBuilder) buildCU(s *shader) {
	cuBuilder := cu.MakeBuilder().
		WithEngine(b.engine).WithFreq(b.freq)

	cuName := fmt.Sprintf("%s.CU[0]", b.name)
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

	name := fmt.Sprintf("%s.VAddrTrans[0]", b.name)
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

	name := fmt.Sprintf("%s.VROB[0]", b.name)
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

	name := fmt.Sprintf("%s.IROB[0]", b.name)
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

	name := fmt.Sprintf("%s.SROB[0]", b.name)
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

	cu.VectorMemModules = &mem.SingleLowModuleFinder{
		LowModule: rob.GetPortByName("Top"),
	}
	b.connectWithDirectConnection(cu.ToVectorMem, rob.GetPortByName("Top"), 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)
}

func (b *shaderBuilder) connectScalarMem(s *shader) {
	cu := s.computeUnit
	rob := s.scalarRoB
	at := s.scalarAddressTranslator

	robTopPort := rob.GetPortByName("Top")
	cu.InstMem = rob.GetPortByName("Top")
	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(robTopPort, 8)
	conn.PlugIn(cu.ToInstMem, 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)
}

func (b *shaderBuilder) connectInstMem(s *shader) {
	cu := s.computeUnit
	rob := s.instructionRoB
	at := s.instructionAddressTranslator

	robTopPort := rob.GetPortByName("Top")
	cu.InstMem = rob.GetPortByName("Top")
	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(robTopPort, 8)
	conn.PlugIn(cu.ToInstMem, 8)

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)
}

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

func (b shaderBuilder) withNumCU(n int) shaderBuilder {
	b.numCU = n
	return b
}

func (b shaderBuilder) withLog2CachelineSize(
	log2Size uint64,
) shaderBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

func (b shaderBuilder) withLog2PageSize(
	log2Size uint64,
) shaderBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b shaderBuilder) withIsaDebugging() shaderBuilder {
	b.isaDebugging = true
	return b
}

func (b shaderBuilder) withVisTracer(
	visTracer tracing.Tracer,
) shaderBuilder {
	b.visTracer = visTracer
	return b
}

func (b shaderBuilder) withMemTracer(
	memTracer tracing.Tracer,
) shaderBuilder {
	b.memTracer = memTracer
	return b
}
