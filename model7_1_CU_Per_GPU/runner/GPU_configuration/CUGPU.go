package GPU_configutation

import (
	"fmt"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/sim/bottleneckanalysis"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v2/dram"
	"gitlab.com/akita/mem/v3/idealmemcontroller"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mgpusim/rdma"
	"gitlab.com/akita/mgpusim/v3/timing/cp"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/pagemigrationcontroller"
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
	numMemoryBank                  int
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
	cuAddrTrans         []*addresstranslator.AddressTranslator

	gpuName string
	gpu     *GPU
	gpuID   uint64
	cp      *cp.CommandProcessor
	cus     []*cu.ComputeUnit
	drams   []*dram.MemController

	lowModuleFinderForPMC   *mem.InterleavedLowModuleFinder
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Engine
	pageMigrationController *pagemigrationcontroller.PageMigrationController
	globalStorage           *mem.Storage

	cuToTAConnection   *sim.DirectConnection
	cpToDRAMConnection *sim.DirectConnection
	internalConn       *sim.DirectConnection
}

func MakeCULevelGPUBuider() cULevelGPUBuilder {
	builder := cULevelGPUBuilder{
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

func (builder cULevelGPUBuilder) Build(name string, id uint64) *GPU {
	builder.createGPU(name, id)
	builder.buildSAs()
	builder.buildDRAMControllers()
	builder.buildCP()

	builder.connectCP()
	return builder.gpu
}

func (builder *cULevelGPUBuilder) createGPU(name string, id uint64) {
	builder.gpuName = name

	builder.gpu = &GPU{}
	builder.gpu.Domain = sim.NewDomain(builder.gpuName)
	builder.gpuID = id
}

// build shader array
func (b *cULevelGPUBuilder) populateCUs(sa *shaderArray) {
	for _, cu := range sa.cus {
		b.cus = append(b.cus, cu)
		b.gpu.CUs = append(b.gpu.CUs, cu)

		if b.monitor != nil {
			b.monitor.RegisterComponent(cu)
		}
	}
}

func (builder *cULevelGPUBuilder) buildSA(
	saBuilder shaderArrayBuilder,
	saName string,
) {
	sa := saBuilder.Build(saName)

	builder.populateCUs(&sa)
}

func (builder *cULevelGPUBuilder) buildSAs() {
	saBuilder := makeShaderBuilder().
		withEngine(builder.engine).
		withFreq(builder.frequency).
		withGPUID(builder.gpuID).
		withNumCU(builder.numCUPerShaderArray)

	if builder.enableISADebugging {
		saBuilder = saBuilder.withIsaDebugging()
	}

	if builder.enableVisTracing {
		saBuilder = saBuilder.withVisTracer(builder.visTracer)
	}

	if builder.enableMemoryTracing {
		saBuilder = saBuilder.withMemTracer(builder.memoryTracer)
	}

	for i := 0; i < builder.numShaderArray; i++ {
		saName := fmt.Sprintf("%s.SA[%d]", builder.gpuName, i)
		builder.buildSA(saBuilder, saName)
	}
}

func (builder *cULevelGPUBuilder) populateAddressTranslators(sa *shaderArray) {
	for _, at := range sa.l1vATs {
		builder.cuAddrTrans = append(builder.cuAddrTrans, at)

		if builder.monitor != nil {
			builder.monitor.RegisterComponent(at)
		}
	}
}

// build DRAM Controllers
func (builder *cULevelGPUBuilder) buildDRAMControllers() {
	// memCtrlBuilder := builder.createDramControllerBuilder()

	for i := 0; i < builder.numMemoryBank; i++ {
		// dramName := fmt.Sprintf("%s.DRAM[%d]", builder.gpuName, i)
		// dram := memCtrlBuilder.
		// 	Build(dramName)
		dram := idealmemcontroller.New(
			fmt.Sprintf("%s.DRAM_%d", builder.gpuName, i),
			builder.engine, 512*mem.MB)
		builder.drams = append(builder.drams, dram)
		builder.gpu.MemControllers = append(builder.gpu.MemControllers, dram)

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
}

// build Command Processor
func (builder *cULevelGPUBuilder) buildCP() {
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

func (builder *cULevelGPUBuilder) buildDMAEngine() {
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

func (builder *cULevelGPUBuilder) buildRDMAEngine() {
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

func (builder *cULevelGPUBuilder) buildPageMigrationController() {
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
func (builder *cULevelGPUBuilder) connectCP() {
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

	builder.connectCPWithCUs()
	builder.connectCPWithDRAM()
}

func (builder *cULevelGPUBuilder) connectCPWithCUs() {
	for _, cu := range builder.cus {
		builder.cp.RegisterCU(cu)
		builder.internalConn.PlugIn(cu.ToACE, 1)
		builder.internalConn.PlugIn(cu.ToCP, 1)
	}
}

func (builder *cULevelGPUBuilder) connectAddressTranslatorwithCP() {
	for _, addressTranslator := range builder.cuAddrTrans {
		ctrlPort := addressTranslator.GetPortByName("Control")
		builder.cp.AddressTranslators = append(builder.cp.AddressTranslators, ctrlPort)
		builder.internalConn.PlugIn(ctrlPort, 1)
	}
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
func (builder *cULevelGPUBuilder) populateExternalPorts() {
	builder.gpu.Domain.AddPort("CommandProcessor", builder.cp.ToDriver)
	builder.gpu.Domain.AddPort("RDMA", builder.rdmaEngine.ToOutside)
	builder.gpu.Domain.AddPort("PageMigrationController",
		builder.pageMigrationController.GetPortByName("Remote"))

	// for i, l2TLB := range b.l2TLBs {
	// 	name := fmt.Sprintf("Translation_%02d", i)
	// 	b.gpu.Domain.AddPort(name, l2TLB.GetPortByName("Bottom"))
	// }
}

// expose parameters to outside //
func (builder cULevelGPUBuilder) WithFrequency(frequency sim.Freq) cULevelGPUBuilder {
	builder.frequency = frequency
	return builder
}

func (builder cULevelGPUBuilder) WithMemAddrOffset(offset uint64) cULevelGPUBuilder {
	builder.memoryAddressOffset = offset
	return builder
}

func (builder cULevelGPUBuilder) WithMMU(mmu *mmu.MMU) cULevelGPUBuilder {
	builder.mmu = mmu
	return builder
}

func (builder cULevelGPUBuilder) WithNumMemoryBank(number int) cULevelGPUBuilder {
	builder.numberMemoryBank = number
	return builder
}

func (builder cULevelGPUBuilder) WithLog2MemoryBankInterleavingSize(n uint64) cULevelGPUBuilder {
	builder.log2MemoryBankInterleavingSize = n
	return builder
}

// WithVisTracer applies a tracer to trace all the tasks of all the GPU
// components
func (builder cULevelGPUBuilder) xWithVisTracer(t tracing.Tracer) cULevelGPUBuilder {
	builder.enableVisTracing = true
	builder.visTracer = t
	return builder
}

// WithNoCTracer applies a tracer to trace all the tasks of the Network-on-Chip.
func (builder cULevelGPUBuilder) WithNoCTracer(t tracing.Tracer) cULevelGPUBuilder {
	builder.enableNoCTracing = true
	builder.nocTracer = t
	return builder
}

// WithMemTracer applies a tracer to trace the memory transactions.
func (builder cULevelGPUBuilder) WithMemTracer(t tracing.Tracer) cULevelGPUBuilder {
	builder.enableMemoryTracing = true
	builder.memoryTracer = t
	return builder
}

// WithISADebugging enables the GPU to dump instruction execution information.
func (builder cULevelGPUBuilder) WithISADebugging() cULevelGPUBuilder {
	builder.enableISADebugging = true
	return builder
}

// WithLog2PageSize sets the page size with the power of 2.
func (builder cULevelGPUBuilder) WithLog2PageSize(log2PageSize uint64) cULevelGPUBuilder {
	builder.log2PageSize = log2PageSize
	return builder
}

// WithMonitor sets the monitor to use.
func (builder cULevelGPUBuilder) WithMonitor(m *monitoring.Monitor) cULevelGPUBuilder {
	builder.monitor = m
	return builder
}

// WithBufferAnalyzer sets the buffer analyzer to use.
func (builder cULevelGPUBuilder) WithBufferAnalyzer(a *bottleneckanalysis.BufferAnalyzer) cULevelGPUBuilder {
	builder.bufferAnalyzer = a
	return builder
}

// WithDRAMSize sets the sum size of all SRAMs in the GPU.
func (builder cULevelGPUBuilder) WithMemorySize(s uint64) cULevelGPUBuilder {
	builder.memorySize = s
	return builder
}

// WithGlobalStorage lets the GPU to build to use the externally provided
// storage.
func (builder cULevelGPUBuilder) WithGlobalStorage(storage *mem.Storage) cULevelGPUBuilder {
	builder.globalStorage = storage
	return builder
}

func (builder cULevelGPUBuilder) WithDRAMSize(size uint64) cULevelGPUBuilder {
	builder.dramSize = size
	return builder
}

// |CU|    |CU|    |CU|                  |CU|
//  |        |       |                     |
// |AT|    |AT|    |AT|       ...        |AT|
// |               DRAM                     |
