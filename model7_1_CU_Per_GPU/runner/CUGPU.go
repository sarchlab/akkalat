package runner

import (
	"fmt"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/sim/bottleneckanalysis"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/idealmemcontroller"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mgpusim/v3/timing/cp"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/pagemigrationcontroller"
	"gitlab.com/akita/mgpusim/v3/timing/rdma"
	rob2 "gitlab.com/akita/mgpusim/v3/timing/rob"
)

type CULevelGPUBuilder struct {
	engine                         sim.Engine
	frequency                      sim.Freq
	memoryAddressOffset            uint64
	mmu                            *mmu.MMU
	numberMemoryBank               int
	memorySize                     uint64
	log2PageSize                   uint64
	log2MemoryBankInterleavingSize uint64
	numShaderArray                 int
	numCUPerShaderArray            int
	dramSize                       uint64

	enableISADebugging  bool
	enableVisTracing    bool
	enableNoCTracing    bool
	enableMemoryTracing bool
	visTracer           tracing.Tracer
	nocTracer           tracing.Tracer
	memoryTracer        tracing.Tracer
	monitor             *monitoring.Monitor
	bufferAnalyzer      *bottleneckanalysis.BufferAnalyzer
	// cuAddrTrans         []*addresstranslator.AddressTranslator

	vectorReorderBuffer      *rob2.ReorderBuffer
	instructionReorderBuffer *rob2.ReorderBuffer
	scalarReorderBuffer      *rob2.ReorderBuffer

	vectorAddressTranslator      *addresstranslator.AddressTranslator
	instructionAddressTranslator *addresstranslator.AddressTranslator
	scalarAddressTranslator      *addresstranslator.AddressTranslator

	gpuName string
	gpu     *GPU
	gpuID   uint64
	cp      *cp.CommandProcessor
	cu      *cu.ComputeUnit
	dram    *idealmemcontroller.Comp

	lowModuleFinderForPMC   *mem.InterleavedLowModuleFinder
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Engine
	pageMigrationController *pagemigrationcontroller.PageMigrationController
	globalStorage           *mem.Storage

	cuToTAConnection   *sim.DirectConnection
	cpToDRAMConnection *sim.DirectConnection
	internalConn       *sim.DirectConnection
	atToDRAMConnection *sim.DirectConnection
}

func MakeCULevelGPUBuider() CULevelGPUBuilder {
	builder := CULevelGPUBuilder{
		numCUPerShaderArray:            1,
		numShaderArray:                 1,
		frequency:                      1 * sim.GHz,
		memoryAddressOffset:            16,
		memorySize:                     4 * mem.GB,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 12,
	}

	return builder
}

func (builder CULevelGPUBuilder) Build(name string, id uint64) *GPU {
	builder.createGPU(name, id)
	builder.buildShader()
	builder.buildDRAMControllers()
	builder.buildCP()

	builder.connectCP()
	builder.connectATToDRAM()

	builder.populateExternalPorts()
	return builder.gpu
}

func (builder *CULevelGPUBuilder) createGPU(name string, id uint64) {
	builder.gpuName = name

	builder.gpu = &GPU{}
	builder.gpu.Domain = sim.NewDomain(builder.gpuName)
	builder.gpuID = id
}

// build shader
func (builder *CULevelGPUBuilder) buildShader() {
	shaderBuilder := makeShaderBuilder().
		withEngine(builder.engine).
		withFreq(builder.frequency).
		withGPUID(builder.gpuID).
		withNumCU(builder.numCUPerShaderArray)

	shaderName := fmt.Sprintf("%s.Shader[0]", builder.gpuName)
	shader := shaderBuilder.Build(shaderName)

	builder.populateCU(&shader)
	builder.populateROB(&shader)
	builder.populateVectorAddressTranslator(&shader)
	builder.populateScalerMemoryHierarchy(&shader)
	builder.populateInstMemoryHierarchy(&shader)

	if builder.enableISADebugging {
		shaderBuilder = shaderBuilder.withIsaDebugging()
	}

	if builder.enableVisTracing {
		shaderBuilder = shaderBuilder.withVisTracer(builder.visTracer)
	}

	if builder.enableMemoryTracing {
		shaderBuilder = shaderBuilder.withMemTracer(builder.memoryTracer)
	}
}

func (builder *CULevelGPUBuilder) populateCU(s *shader) {
	builder.cu = s.computeUnit
	builder.gpu.CU = s.computeUnit

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(s.computeUnit)
	}
}

func (builder *CULevelGPUBuilder) populateROB(s *shader) {
	builder.vectorReorderBuffer = s.vectorRoB

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(builder.vectorReorderBuffer)
	}
}

func (builder *CULevelGPUBuilder) populateVectorAddressTranslator(s *shader) {
	builder.vectorAddressTranslator = s.vectorAddressTranslator

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(builder.vectorAddressTranslator)
	}
}

func (builder *CULevelGPUBuilder) populateInstMemoryHierarchy(s *shader) {
	builder.instructionAddressTranslator = s.instructionAddressTranslator
	builder.instructionReorderBuffer = s.instructionRoB

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(s.instructionAddressTranslator)
		builder.monitor.RegisterComponent(s.instructionRoB)
	}
}

func (builder *CULevelGPUBuilder) populateScalerMemoryHierarchy(s *shader) {
	builder.scalarAddressTranslator = s.scalarAddressTranslator
	builder.scalarReorderBuffer = s.scalarRoB

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(s.scalarAddressTranslator)
		builder.monitor.RegisterComponent(s.scalarRoB)
	}
}

// build

// build DRAM Controllers
func (builder *CULevelGPUBuilder) buildDRAMControllers() {
	// memCtrlBuilder := builder.createDramControllerBuilder()

	dram := idealmemcontroller.New(
		fmt.Sprintf("%s.DRAM[0]", builder.gpuName),
		builder.engine, 512*mem.MB)
	builder.dram = dram
	builder.gpu.MemController = dram

	if builder.enableVisTracing {
		tracing.CollectTrace(dram, builder.visTracer)
	}

	if builder.enableMemoryTracing {
		tracing.CollectTrace(dram, builder.memoryTracer)
	}

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(dram)
	}
}

// build Command Processor
func (builder *CULevelGPUBuilder) buildCP() {
	build := cp.MakeBuilder().
		WithEngine(builder.engine).
		WithFreq(builder.frequency).
		WithMonitor(builder.monitor)

	if builder.enableVisTracing {
		build = build.WithVisTracer(builder.visTracer)
	}

	builder.cp = build.Build(builder.gpuName + ".CommandProcessor")
	builder.gpu.CommandProcessor = builder.cp

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(builder.cp)
	}

	builder.buildDMAEngine()
	builder.buildRDMAEngine()
	builder.buildPageMigrationController()
}

func (builder *CULevelGPUBuilder) buildDMAEngine() {
	builder.dmaEngine = cp.NewDMAEngine(
		fmt.Sprintf("%s.DMA", builder.gpuName),
		builder.engine,
		nil)

	if builder.enableVisTracing {
		tracing.CollectTrace(builder.dmaEngine, builder.visTracer)
	}

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(builder.dmaEngine)
	}
}

func (builder *CULevelGPUBuilder) buildRDMAEngine() {
	builder.rdmaEngine = rdma.NewEngine(
		fmt.Sprintf("%s.RDMA", builder.gpuName),
		builder.engine,
		nil,
		nil,
	)
	builder.gpu.RDMAEngine = builder.rdmaEngine

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(builder.rdmaEngine)
	}

	if builder.enableVisTracing {
		tracing.CollectTrace(builder.rdmaEngine, builder.visTracer)
	}
}

func (builder *CULevelGPUBuilder) buildPageMigrationController() {
	builder.pageMigrationController =
		pagemigrationcontroller.NewPageMigrationController(
			fmt.Sprintf("%s.PMC", builder.gpuName),
			builder.engine,
			builder.lowModuleFinderForPMC,
			nil)
	builder.gpu.PMC = builder.pageMigrationController

	if builder.monitor != nil {
		builder.monitor.RegisterComponent(builder.pageMigrationController)
	}
}

// connect Command Processor with other components
func (builder *CULevelGPUBuilder) connectCP() {
	builder.internalConn = sim.NewDirectConnection(
		builder.gpuName+".InternalConn", builder.engine, builder.frequency)

	builder.internalConn.PlugIn(builder.cp.ToDriver, 1)
	builder.internalConn.PlugIn(builder.cp.ToDMA, 128)
	builder.internalConn.PlugIn(builder.cp.ToCUs, 128)
	builder.internalConn.PlugIn(builder.cp.ToAddressTranslators, 128)
	builder.internalConn.PlugIn(builder.cp.ToRDMA, 4)
	builder.internalConn.PlugIn(builder.cp.ToPMC, 4)

	builder.cp.RDMA = builder.rdmaEngine.CtrlPort
	builder.internalConn.PlugIn(builder.cp.RDMA, 1)

	builder.cp.DMAEngine = builder.dmaEngine.ToCP
	builder.internalConn.PlugIn(builder.dmaEngine.ToCP, 1)

	pmcControlPort := builder.pageMigrationController.GetPortByName("Control")
	builder.cp.PMC = pmcControlPort
	builder.internalConn.PlugIn(pmcControlPort, 1)

	builder.connectCPWithCU()
	builder.connectAddressTranslatorwithCP()
}

func (builder *CULevelGPUBuilder) connectCPWithCU() {
	builder.cp.RegisterCU(builder.cu)
	builder.internalConn.PlugIn(builder.cu.ToACE, 1)
	builder.internalConn.PlugIn(builder.cu.ToCP, 1)
}

func (builder *CULevelGPUBuilder) connectAddressTranslatorwithCP() {
	vectorAddressTranslatorCtlPort := builder.vectorAddressTranslator.GetPortByName("Control")
	builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, vectorAddressTranslatorCtlPort)

	scalarAddressTranslatorCtlPort := builder.scalarAddressTranslator.GetPortByName("Control")
	builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, scalarAddressTranslatorCtlPort)

	instructionAddressTranslatorCtlPort := builder.instructionAddressTranslator.GetPortByName("Control")
	builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, instructionAddressTranslatorCtlPort)

	vectorROBCtlPort := builder.vectorReorderBuffer.GetPortByName("Control")
	builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, vectorROBCtlPort)

	scalarROBCtlPort := builder.scalarReorderBuffer.GetPortByName("Control")
	builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, scalarROBCtlPort)

	instructionROBCtlPort := builder.instructionReorderBuffer.GetPortByName("Control")
	builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, instructionROBCtlPort)
}

func (builder *CULevelGPUBuilder) connectATToDRAM() {
	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << builder.log2MemoryBankInterleavingSize)
	lowModuleFinder.ModuleForOtherAddresses = builder.rdmaEngine.ToL1
	lowModuleFinder.UseAddressSpaceLimitation = true
	lowModuleFinder.LowAddress = builder.memoryAddressOffset
	lowModuleFinder.HighAddress = builder.memoryAddressOffset + 512*mem.MB

	builder.vectorAddressTranslator.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: builder.dram.GetPortByName("Top"),
	})
	builder.instructionAddressTranslator.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: builder.dram.GetPortByName("Top"),
	})
	builder.scalarAddressTranslator.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: builder.dram.GetPortByName("Top"),
	})

	builder.atToDRAMConnection.PlugIn(builder.dram.GetPortByName("Top"), 64)
	builder.atToDRAMConnection.PlugIn(builder.vectorAddressTranslator.GetPortByName("Bottom"), 64)
	builder.atToDRAMConnection.PlugIn(builder.instructionAddressTranslator.GetPortByName("Bottom"), 64)
	builder.atToDRAMConnection.PlugIn(builder.scalarAddressTranslator.GetPortByName("Bottom"), 64)

	builder.dmaEngine.SetLocalDataSource(lowModuleFinder)
	builder.atToDRAMConnection.PlugIn(builder.dmaEngine.ToMem, 64)

	builder.pageMigrationController.MemCtrlFinder = lowModuleFinder
	builder.atToDRAMConnection.PlugIn(
		builder.pageMigrationController.GetPortByName("LocalMem"), 16)
}

// func (builder *cULevelGPUBuilder) connectCPWithDRAM() {
// 	builder.cpToDRAMConnection = sim.NewDirectConnection(
// 		builder.gpuName+".CPtoDRAM", builder.engine, builder.frequency)

// 	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
// 		1 << builder.log2MemoryBankInterleavingSize)

// 		builder.cpToDRAMConnection.PlugIn(builder.cp.GetPortByName("Bottom"), 64)
// 		l2.SetLowModuleFinder(&mem.SingleLowModuleFinder{
// 			LowModule: builder.drams[i].GetPortByName("Top"),
// 		})
// 	}

// 	for _, dram := range b.drams {
// 		b.l2ToDramConnection.PlugIn(dram.GetPortByName("Top"), 64)
// 		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
// 			dram.GetPortByName("Top"))
// 	}

// 	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
// 	b.l2ToDramConnection.PlugIn(b.dmaEngine.ToMem, 64)

// 	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
// 	b.l2ToDramConnection.PlugIn(
// 		b.pageMigrationController.GetPortByName("LocalMem"), 16)
// }

// populate external ports
func (builder *CULevelGPUBuilder) populateExternalPorts() {
	builder.gpu.Domain.AddPort("CommandProcessor", builder.cp.ToDriver)
	builder.gpu.Domain.AddPort("RDMA", builder.rdmaEngine.ToOutside)
	builder.gpu.Domain.AddPort("PageMigrationController",
		builder.pageMigrationController.GetPortByName("Remote"))
}

// expose parameters to outside //
func (builder CULevelGPUBuilder) WithFrequency(frequency sim.Freq) CULevelGPUBuilder {
	builder.frequency = frequency
	return builder
}

func (builder CULevelGPUBuilder) WithMemAddrOffset(offset uint64) CULevelGPUBuilder {
	builder.memoryAddressOffset = offset
	return builder
}

func (builder CULevelGPUBuilder) WithMMU(mmu *mmu.MMU) CULevelGPUBuilder {
	builder.mmu = mmu
	return builder
}

func (builder CULevelGPUBuilder) WithNumMemoryBank(number int) CULevelGPUBuilder {
	builder.numberMemoryBank = number
	return builder
}

func (builder CULevelGPUBuilder) WithLog2MemoryBankInterleavingSize(n uint64) CULevelGPUBuilder {
	builder.log2MemoryBankInterleavingSize = n
	return builder
}

// WithVisTracer applies a tracer to trace all the tasks of all the GPU
// components
func (builder CULevelGPUBuilder) WithVisTracer(t tracing.Tracer) CULevelGPUBuilder {
	builder.enableVisTracing = true
	builder.visTracer = t
	return builder
}

// WithNoCTracer applies a tracer to trace all the tasks of the Network-on-Chip.
func (builder CULevelGPUBuilder) WithNoCTracer(t tracing.Tracer) CULevelGPUBuilder {
	builder.enableNoCTracing = true
	builder.nocTracer = t
	return builder
}

// WithMemTracer applies a tracer to trace the memory transactions.
func (builder CULevelGPUBuilder) WithMemTracer(t tracing.Tracer) CULevelGPUBuilder {
	builder.enableMemoryTracing = true
	builder.memoryTracer = t
	return builder
}

// WithISADebugging enables the GPU to dump instruction execution information.
func (builder CULevelGPUBuilder) WithISADebugging() CULevelGPUBuilder {
	builder.enableISADebugging = true
	return builder
}

// WithLog2PageSize sets the page size with the power of 2.
func (builder CULevelGPUBuilder) WithLog2PageSize(log2PageSize uint64) CULevelGPUBuilder {
	builder.log2PageSize = log2PageSize
	return builder
}

// WithMonitor sets the monitor to use.
func (builder CULevelGPUBuilder) WithMonitor(m *monitoring.Monitor) CULevelGPUBuilder {
	builder.monitor = m
	return builder
}

// WithBufferAnalyzer sets the buffer analyzer to use.
func (builder CULevelGPUBuilder) WithBufferAnalyzer(a *bottleneckanalysis.BufferAnalyzer) CULevelGPUBuilder {
	builder.bufferAnalyzer = a
	return builder
}

// WithDRAMSize sets the sum size of all SRAMs in the GPU.
func (builder CULevelGPUBuilder) WithMemorySize(s uint64) CULevelGPUBuilder {
	builder.memorySize = s
	return builder
}

// WithGlobalStorage lets the GPU to build to use the externally provided
// storage.
func (builder CULevelGPUBuilder) WithGlobalStorage(storage *mem.Storage) CULevelGPUBuilder {
	builder.globalStorage = storage
	return builder
}

func (builder CULevelGPUBuilder) WithDRAMSize(size uint64) CULevelGPUBuilder {
	builder.dramSize = size
	return builder
}

func (b CULevelGPUBuilder) WithEngine(engine sim.Engine) CULevelGPUBuilder {
	b.engine = engine
	return b
}

// |CU|    |CU|    |CU|                  |CU|
//  |        |       |                     |
// |AT|    |AT|    |AT|       ...        |AT|
// |               DRAM                     |
