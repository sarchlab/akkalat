package runner

import (
	"fmt"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/sim/bottleneckanalysis"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/timing/cp"
	"gitlab.com/akita/mgpusim/v3/timing/pagemigrationcontroller"
	"gitlab.com/akita/mgpusim/v3/timing/rdma"
)

// WaferScaleGPUBuilder can build Wafer-Scale GPUs.
type WaferScaleGPUBuilder struct {
	engine                         sim.Engine
	freq                           sim.Freq
	memAddrOffset                  uint64
	mmu                            *mmu.MMU
	numMemoryBank                  int
	memorySize                     uint64
	log2PageSize                   uint64
	log2CacheLineSize              uint64
	log2MemoryBankInterleavingSize uint64

	enableISADebugging bool
	enableVisTracing   bool
	enableNoCTracing   bool
	enableMemTracing   bool
	visTracer          tracing.Tracer
	nocTracer          tracing.Tracer
	memTracer          tracing.Tracer
	monitor            *monitoring.Monitor
	bufferAnalyzer     *bottleneckanalysis.BufferAnalyzer

	gpuName string
	gpu     *GPU
	gpuID   uint64
	cp      *cp.CommandProcessor

	lowModuleFinderForL1    *mem.InterleavedLowModuleFinder
	lowModuleFinderForPMC   *mem.InterleavedLowModuleFinder
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Engine
	pageMigrationController *pagemigrationcontroller.PageMigrationController
	globalStorage           *mem.Storage

	periphConn *sim.DirectConnection

	tileWidth  int // use CLI options in runner.go
	tileHeight int
	mesh       *mesh
	ToMesh     []sim.Port
	l2TLB      *tlb.TLB

	connectionCount int
}

// MakeWaferScaleGPUBuilder provides a GPU builder that can builds the Wafer-Scale GPU.
func MakeWaferScaleGPUBuilder() WaferScaleGPUBuilder {
	b := WaferScaleGPUBuilder{
		freq:                           1 * sim.GHz,
		numMemoryBank:                  16,
		log2CacheLineSize:              6,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 12,
		memorySize:                     4 * mem.GB,
	}
	return b
}

// WithEngine sets the engine that the GPU use.
func (b WaferScaleGPUBuilder) WithEngine(
	engine sim.Engine,
) WaferScaleGPUBuilder {
	b.engine = engine
	return b
}

// WithFreq sets the frequency that the GPU works at.
func (b WaferScaleGPUBuilder) WithFreq(freq sim.Freq) WaferScaleGPUBuilder {
	b.freq = freq
	return b
}

// WithMemAddrOffset sets the address of the first byte of the GPU to build.
func (b WaferScaleGPUBuilder) WithMemAddrOffset(
	offset uint64,
) WaferScaleGPUBuilder {
	b.memAddrOffset = offset
	return b
}

// WithMMU sets the MMU component that provides the address translation service
// for the GPU.
func (b WaferScaleGPUBuilder) WithMMU(mmu *mmu.MMU) WaferScaleGPUBuilder {
	b.mmu = mmu
	return b
}

// WithNumMemoryBank sets the number of L2 cache modules and number of memory
// controllers in each GPU.
func (b WaferScaleGPUBuilder) WithNumMemoryBank(n int) WaferScaleGPUBuilder {
	b.numMemoryBank = n
	return b
}

// WithLog2MemoryBankInterleavingSize sets the number of consecutive bytes that
// are guaranteed to be on a memory bank.
func (b WaferScaleGPUBuilder) WithLog2MemoryBankInterleavingSize(
	n uint64,
) WaferScaleGPUBuilder {
	b.log2MemoryBankInterleavingSize = n
	return b
}

// WithVisTracer applies a tracer to trace all the tasks of all the GPU
// components
func (b WaferScaleGPUBuilder) WithVisTracer(
	t tracing.Tracer,
) WaferScaleGPUBuilder {
	b.enableVisTracing = true
	b.visTracer = t
	return b
}

// WithNoCTracer applies a tracer to trace all the tasks of the Network-on-Chip.
func (b WaferScaleGPUBuilder) WithNoCTracer(
	t tracing.Tracer,
) WaferScaleGPUBuilder {
	b.enableNoCTracing = true
	b.nocTracer = t
	return b
}

// WithMemTracer applies a tracer to trace the memory transactions.
func (b WaferScaleGPUBuilder) WithMemTracer(
	t tracing.Tracer,
) WaferScaleGPUBuilder {
	b.enableMemTracing = true
	b.memTracer = t
	return b
}

// WithISADebugging enables the GPU to dump instruction execution information.
func (b WaferScaleGPUBuilder) WithISADebugging() WaferScaleGPUBuilder {
	b.enableISADebugging = true
	return b
}

// WithLog2CacheLineSize sets the cache line size with the power of 2.
func (b WaferScaleGPUBuilder) WithLog2CacheLineSize(
	log2CacheLine uint64,
) WaferScaleGPUBuilder {
	b.log2CacheLineSize = log2CacheLine
	return b
}

// WithLog2PageSize sets the page size with the power of 2.
func (b WaferScaleGPUBuilder) WithLog2PageSize(
	log2PageSize uint64,
) WaferScaleGPUBuilder {
	b.log2PageSize = log2PageSize
	return b
}

// WithMonitor sets the monitor to use.
func (b WaferScaleGPUBuilder) WithMonitor(
	m *monitoring.Monitor,
) WaferScaleGPUBuilder {
	b.monitor = m
	return b
}

// WithBufferAnalyzer sets the buffer analyzer to use.
func (b WaferScaleGPUBuilder) WithBufferAnalyzer(
	a *bottleneckanalysis.BufferAnalyzer,
) WaferScaleGPUBuilder {
	b.bufferAnalyzer = a
	return b
}

// WithDRAMSize sets the sum size of all SRAMs in the GPU.
func (b WaferScaleGPUBuilder) WithMemorySize(s uint64) WaferScaleGPUBuilder {
	b.memorySize = s
	return b
}

// WithGlobalStorage lets the GPU to build to use the externally provided
// storage.
func (b WaferScaleGPUBuilder) WithGlobalStorage(
	storage *mem.Storage,
) WaferScaleGPUBuilder {
	b.globalStorage = storage
	return b
}

// WithTileWidth sets the number of tiles horizontally.
func (b WaferScaleGPUBuilder) WithTileWidth(w int) WaferScaleGPUBuilder {
	b.tileWidth = w
	return b
}

// WithTileHeight sets the number of tiles vertically.
func (b WaferScaleGPUBuilder) WithTileHeight(h int) WaferScaleGPUBuilder {
	b.tileHeight = h
	return b
}

// Build creates a pre-configure GPU similar to the AMD R9 Nano GPU.
func (b WaferScaleGPUBuilder) Build(name string, id uint64) *GPU {
	b.createGPU(name, id)
	b.buildCP()
	b.buildL2TLB()

	b.connectPeriphComponents()

	b.buildMesh(name)

	b.populateExternalPorts()

	return b.gpu
}

func (b *WaferScaleGPUBuilder) populateExternalPorts() {
	b.gpu.Domain.AddPort("CommandProcessor", b.cp.ToDriver)
	b.gpu.Domain.AddPort("RDMA", b.rdmaEngine.ToOutside)
	b.gpu.Domain.AddPort("PageMigrationController",
		b.pageMigrationController.GetPortByName("Remote"))
	b.gpu.Domain.AddPort("Translation", b.l2TLB.GetPortByName("Bottom"))
}

func (b *WaferScaleGPUBuilder) createGPU(name string, id uint64) {
	b.gpuName = name

	b.gpu = &GPU{}
	b.gpu.Domain = sim.NewDomain(b.gpuName)
	b.gpuID = id
}

func (b *WaferScaleGPUBuilder) connectPeriphComponents() {
	b.periphConn = sim.NewDirectConnection(
		b.gpuName+"PeriphConn", b.engine, b.freq)

	/* CP <-> Driver, RDMA, PMC */
	b.periphConn.PlugIn(b.cp.ToDriver, 1)
	b.periphConn.PlugIn(b.cp.ToRDMA, 4)
	b.periphConn.PlugIn(b.cp.ToPMC, 4)

	/* CP <-> Mesh(CUs, TLBs, ATs, ROBs, Caches) */
	b.ToMesh = append(b.ToMesh, b.cp.ToCUs)
	b.ToMesh = append(b.ToMesh, b.cp.ToTLBs)
	b.ToMesh = append(b.ToMesh, b.cp.ToCaches)
	b.ToMesh = append(b.ToMesh, b.cp.ToAddressTranslators)

	/* RDMA(Control) <-> CP */
	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.periphConn.PlugIn(b.cp.RDMA, 1)

	/* DMA(Control) <-> Mesh(CP.ToDMA) */
	/* DMA(ToMem) <-> Mesh(SRAMs) */
	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.connectWithDirectConnection(b.cp.ToDMA, b.dmaEngine.ToCP, 128)

	/* PMC(Control) <-> CP */
	/* PMC(LocalMem) <-> Mesh(SRAM) */
	pmcControlPort := b.pageMigrationController.GetPortByName("Control")
	b.cp.PMC = pmcControlPort
	b.periphConn.PlugIn(pmcControlPort, 1)
	b.ToMesh = append(b.ToMesh,
		b.pageMigrationController.GetPortByName("LocalMem"))

	b.ToMesh = append(b.ToMesh, b.dmaEngine.ToMem)

	/* L2TLB(Top) <-> L1TLBs(Mesh -> Bottom) */
	/* L2TLB(Bottom) <-> MMU ; definied in populateExternalPorts */
	/* L2TLB(Control) <-> CP */
	l2TLBCtrlPort := b.l2TLB.GetPortByName("Control")
	b.periphConn.PlugIn(l2TLBCtrlPort, 1)
	b.cp.TLBs = append(b.cp.TLBs, l2TLBCtrlPort)
	b.ToMesh = append(b.ToMesh, b.l2TLB.GetPortByName("Top"))
}

func (b *WaferScaleGPUBuilder) buildMesh(name string) {
	meshBuilder := makeMeshBuilder().
		withGPUID(b.gpuID).
		withGPUName(b.gpuName).
		withGPUPtr(b.gpu).
		withCP(b.cp).
		withL2TLB(b.l2TLB).
		withEngine(b.engine).
		withFreq(b.freq).
		WithMemAddrOffset(b.memAddrOffset).
		withLog2CachelineSize(b.log2CacheLineSize).
		withLog2PageSize(b.log2PageSize).
		withTileWidth(b.tileWidth).
		withTileHeight(b.tileHeight).
		withLog2MemoryBankInterleavingSize(b.log2MemoryBankInterleavingSize).
		withDMAEngine(b.dmaEngine).
		withPageMigrationController(b.pageMigrationController).
		withGlobalStorage(b.globalStorage)

	if b.enableISADebugging {
		meshBuilder = meshBuilder.WithISADebugging()
	}

	if b.enableVisTracing {
		meshBuilder = meshBuilder.WithVisTracer(b.visTracer)
	}

	if b.enableNoCTracing {
		meshBuilder = meshBuilder.WithNoCTracer(b.nocTracer)
	}

	if b.monitor != nil {
		meshBuilder = meshBuilder.WithMonitor(b.monitor)
	}

	if b.bufferAnalyzer != nil {
		meshBuilder = meshBuilder.WithBufferAnalyzer(b.bufferAnalyzer)
	}

	b.mesh = meshBuilder.Build(name, b.ToMesh)
}

func (b *WaferScaleGPUBuilder) buildRDMAEngine() {
	b.rdmaEngine = rdma.NewEngine(
		fmt.Sprintf("%s.RDMA", b.gpuName),
		b.engine,
		b.lowModuleFinderForL1,
		nil,
	)
	b.gpu.RDMAEngine = b.rdmaEngine

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.rdmaEngine)
	}
}

func (b *WaferScaleGPUBuilder) buildPageMigrationController() {
	b.pageMigrationController =
		pagemigrationcontroller.NewPageMigrationController(
			fmt.Sprintf("%s.PMC", b.gpuName),
			b.engine,
			b.lowModuleFinderForPMC,
			nil)
	b.gpu.PMC = b.pageMigrationController

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.pageMigrationController)
	}
}

func (b *WaferScaleGPUBuilder) buildDMAEngine() {
	b.dmaEngine = cp.NewDMAEngine(
		fmt.Sprintf("%s.DMA", b.gpuName),
		b.engine,
		nil)

	if b.enableVisTracing {
		tracing.CollectTrace(b.dmaEngine, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.dmaEngine)
	}
}

func (b *WaferScaleGPUBuilder) buildCP() {
	builder := cp.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithMonitor(b.monitor)

	if b.enableVisTracing {
		builder = builder.WithVisTracer(b.visTracer)
	}

	b.cp = builder.Build(b.gpuName + ".CommandProcessor")
	b.gpu.CommandProcessor = b.cp

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.cp)
	}

	b.buildDMAEngine()
	b.buildRDMAEngine()
	b.buildPageMigrationController()
}

func (b *WaferScaleGPUBuilder) buildL2TLB() {
	numWays := 64
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumWays(numWays).
		WithNumSets(int(b.memorySize / (1 << b.log2PageSize) / uint64(numWays))).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(1024).
		WithPageSize(1 << b.log2PageSize).
		WithLowModule(b.mmu.GetPortByName("Top"))

	b.l2TLB = builder.Build(fmt.Sprintf("%s.L2TLB", b.gpuName))
	b.gpu.L2TLBs = append(b.gpu.L2TLBs, b.l2TLB)

	if b.enableVisTracing {
		tracing.CollectTrace(b.l2TLB, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.l2TLB)
	}
}

func (b *WaferScaleGPUBuilder) NumCU() int {
	return b.tileHeight * b.tileWidth
}

func (b *WaferScaleGPUBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	name := fmt.Sprintf("%s.Conn[%d]", b.gpuName, b.connectionCount)
	b.connectionCount++

	conn := sim.NewDirectConnection(
		name,
		b.engine, b.freq,
	)

	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
}
