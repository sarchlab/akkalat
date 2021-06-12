package runner

import (
	"fmt"

	"gitlab.com/akita/akita/v2/monitoring"
	"gitlab.com/akita/akita/v2/sim"
	"gitlab.com/akita/mem/v2/dram"
	"gitlab.com/akita/mem/v2/mem"
	"gitlab.com/akita/mem/v2/vm/mmu"
	"gitlab.com/akita/mem/v2/vm/tlb"
	"gitlab.com/akita/mgpusim/v2/pagemigrationcontroller"
	"gitlab.com/akita/mgpusim/v2/rdma"
	"gitlab.com/akita/mgpusim/v2/timing/cp"
	"gitlab.com/akita/util/v2/tracing"
)

// WaferScaleGPUBuilder can build wafer-scale GPUs.
type WaferScaleGPUBuilder struct {
	engine                         sim.Engine
	freq                           sim.Freq
	memAddrOffset                  uint64
	mmu                            *mmu.MMUImpl
	numCUPerShaderArray            int
	numMemoryBank                  int
	l2CacheSize                    uint64
	log2PageSize                   uint64
	log2CacheLineSize              uint64
	log2MemoryBankInterleavingSize uint64

	enableISADebugging bool
	enableMemTracing   bool
	enableVisTracing   bool
	visTracer          tracing.Tracer
	memTracer          tracing.Tracer
	monitor            *monitoring.Monitor

	gpuName                 string
	gpu                     *GPU
	gpuID                   uint64
	cp                      *cp.CommandProcessor
	m                       *MeshComponent
	drams                   []*dram.MemController
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Engine
	pageMigrationController *pagemigrationcontroller.PageMigrationController

	periphConn *sim.DirectConnection
	// dmaToDramConnection *sim.DirectConnection

	lowModuleFinderForL1  *mem.InterleavedLowModuleFinder
	lowModuleFinderForL2  *mem.InterleavedLowModuleFinder
	lowModuleFinderForPMC *mem.InterleavedLowModuleFinder

	tileWidth   int
	tileHeight  int
	periphPorts []sim.Port
	l2TLB       *tlb.TLB
}

// MakeWaferScaleGPUBuilder creates a WaferScaleGPUBuilder.
func MakeWaferScaleGPUBuilder() WaferScaleGPUBuilder {
	return WaferScaleGPUBuilder{
		freq:                           1 * sim.GHz,
		tileWidth:                      2,
		tileHeight:                     2,
		numCUPerShaderArray:            4,
		numMemoryBank:                  16,
		log2CacheLineSize:              6,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 12,
		l2CacheSize:                    2 * mem.MB,
	}
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
func (b WaferScaleGPUBuilder) WithMMU(mmu *mmu.MMUImpl) WaferScaleGPUBuilder {
	b.mmu = mmu
	return b
}

// WithNumMemoryBank sets the number of L2 cache modules and number of memory
// controllers in each GPU.
func (b WaferScaleGPUBuilder) WithNumMemoryBank(n int) WaferScaleGPUBuilder {
	b.numMemoryBank = n
	return b
}

// WithNumCUPerShaderArray sets the number of CU and number of L1V caches in
// each Shader Array.
func (b WaferScaleGPUBuilder) WithNumCUPerShaderArray(
	n int,
) WaferScaleGPUBuilder {
	b.numCUPerShaderArray = n
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

// WithMonitor sets the monitor that monitoring the execution of the GPU's
// internal components.
func (b WaferScaleGPUBuilder) WithMonitor(
	m *monitoring.Monitor,
) WaferScaleGPUBuilder {
	b.monitor = m
	return b
}

// WithL2CacheSize set the total L2 cache size. The size of the L2 cache is
// split between memory banks.
func (b WaferScaleGPUBuilder) WithL2CacheSize(
	size uint64,
) WaferScaleGPUBuilder {
	b.l2CacheSize = size
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

// Build builds a new wafer-scale GPU.
func (b WaferScaleGPUBuilder) Build(name string, id uint64) *GPU {
	b.createGPU(name, id)
	b.buildCP()
	b.buildL2TLB()
	b.buildDRAMControllers()

	b.connectDRAMAndDMA()
	b.connectPeriphComponents()

	b.buildMesh(name)
	b.connectL2AndDRAM()
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
		withVisTracer(b.visTracer).
		withMemTracer(b.memTracer).
		withMonitor(b.monitor).
		withTileWidth(b.tileWidth).
		withTileHeight(b.tileHeight).
		withNumCUPerShaderArray(b.numCUPerShaderArray).
		withNumMemoryBank(b.numMemoryBank).
		withLog2MemoryBankInterleavingSize(b.log2MemoryBankInterleavingSize).
		withRDMAEngine(b.rdmaEngine)
	b.m = meshBuilder.Build(name, b.periphPorts)
}

func (b *WaferScaleGPUBuilder) connectPeriphComponents() {
	b.periphConn = sim.NewDirectConnection(
		b.gpuName+"PeriphConn", b.engine, b.freq)

	/* CP <-> Driver, DMA, RDMA, PMC */
	b.periphConn.PlugIn(b.cp.ToDriver, 1)
	b.periphConn.PlugIn(b.cp.ToDMA, 128)
	b.periphConn.PlugIn(b.cp.ToRDMA, 4)
	b.periphConn.PlugIn(b.cp.ToPMC, 4)

	/* CP <-> CUs, TLBs, Caches, ATs, ROBs */
	b.periphPorts = append(b.periphPorts, b.cp.ToCUs)
	b.periphPorts = append(b.periphPorts, b.cp.ToTLBs)
	b.periphPorts = append(b.periphPorts, b.cp.ToCaches)
	b.periphPorts = append(b.periphPorts, b.cp.ToAddressTranslators)

	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.periphConn.PlugIn(b.cp.RDMA, 1)

	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.periphConn.PlugIn(b.dmaEngine.ToCP, 1)

	pmcControlPort := b.pageMigrationController.GetPortByName("Control")
	b.cp.PMC = pmcControlPort
	b.periphConn.PlugIn(pmcControlPort, 1)

	/* L2TLB(Bottom) <-> MMU */
	/* L2TLB(Control) <-> CP */
	l2TLBCtrlPort := b.l2TLB.GetPortByName("Control")
	b.cp.TLBs = append(b.cp.TLBs, l2TLBCtrlPort)
	b.periphPorts = append(b.periphPorts, l2TLBCtrlPort)
	b.periphPorts = append(b.periphPorts, b.l2TLB.GetPortByName("Top"))

}

func (b *WaferScaleGPUBuilder) connectDRAMAndDMA() {
	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)

	for _, dram := range b.drams {
		b.periphPorts = append(b.periphPorts, dram.GetPortByName("Top"))
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			dram.GetPortByName("Top"))
	}

	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
	b.periphPorts = append(b.periphPorts, b.dmaEngine.ToMem)

	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
	b.periphPorts = append(b.periphPorts,
		b.pageMigrationController.GetPortByName("LocalMem"))

	b.periphPorts = append(b.periphPorts, b.rdmaEngine.ToL1)
	b.periphPorts = append(b.periphPorts, b.rdmaEngine.ToL2)
}

func (b *WaferScaleGPUBuilder) connectL2AndDRAM() {
	for i, l2 := range b.m.l2Caches {
		l2.SetLowModuleFinder(&mem.SingleLowModuleFinder{
			LowModule: b.drams[i].GetPortByName("Top"),
		})
	}
}

func (b *WaferScaleGPUBuilder) buildDRAMControllers() {
	memCtrlBuilder := b.createDramControllerBuilder()

	for i := 0; i < b.numMemoryBank; i++ {
		dramName := fmt.Sprintf("%s.DRAM_%d", b.gpuName, i)
		dram := memCtrlBuilder.
			WithInterleavingAddrConversion(
				1<<b.log2MemoryBankInterleavingSize,
				b.numMemoryBank,
				i, b.memAddrOffset, b.memAddrOffset+4*mem.GB,
			).
			Build(dramName)
		// dram := idealmemcontroller.New(
		// 	fmt.Sprintf("%s.DRAM_%d", b.gpuName, i),
		// 	b.engine, 512*mem.MB)
		b.drams = append(b.drams, dram)
		b.gpu.MemControllers = append(b.gpu.MemControllers, dram)

		if b.enableVisTracing {
			tracing.CollectTrace(dram, b.visTracer)
		}

		if b.enableMemTracing {
			tracing.CollectTrace(dram, b.memTracer)
		}

		if b.monitor != nil {
			b.monitor.RegisterComponent(dram)
		}
	}
}

func (b *WaferScaleGPUBuilder) createDramControllerBuilder() dram.Builder {
	memBankSize := 4 * mem.GB / uint64(b.numMemoryBank)
	if 4*mem.GB%uint64(b.numMemoryBank) != 0 {
		panic("GPU memory size is not a multiple of the number of memory banks")
	}

	dramCol := 64
	dramRow := 16384
	dramDeviceWidth := 128
	dramBankSize := dramCol * dramRow * dramDeviceWidth
	dramBank := 4
	dramBankGroup := 4
	dramBusWidth := 256
	dramDevicePerRank := dramBusWidth / dramDeviceWidth
	dramRankSize := dramBankSize * dramDevicePerRank * dramBank
	dramRank := int(memBankSize * 8 / uint64(dramRankSize))

	memCtrlBuilder := dram.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(500 * sim.MHz).
		WithProtocol(dram.HBM).
		WithBurstLength(4).
		WithDeviceWidth(dramDeviceWidth).
		WithBusWidth(dramBusWidth).
		WithNumChannel(1).
		WithNumRank(dramRank).
		WithNumBankGroup(dramBankGroup).
		WithNumBank(dramBank).
		WithNumCol(dramCol).
		WithNumRow(dramRow).
		WithCommandQueueSize(8).
		WithTransactionQueueSize(32).
		WithTCL(7).
		WithTCWL(2).
		WithTRCDRD(7).
		WithTRCDWR(7).
		WithTRP(7).
		WithTRAS(17).
		WithTREFI(1950).
		WithTRRDS(2).
		WithTRRDL(3).
		WithTWTRS(3).
		WithTWTRL(4).
		WithTWR(8).
		WithTCCDS(1).
		WithTCCDL(1).
		WithTRTRS(0).
		WithTRTP(3).
		WithTPPD(2)

	if b.visTracer != nil {
		memCtrlBuilder = memCtrlBuilder.WithAdditionalTracer(b.visTracer)
	}

	return memCtrlBuilder
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

func (b *WaferScaleGPUBuilder) numCU() int {
	return b.numCUPerShaderArray * b.tileWidth * b.tileHeight
}

func (b *WaferScaleGPUBuilder) buildL2TLB() {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumWays(64).
		WithNumSets(64).
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
