package gpuArch

import (
	"fmt"

	"github.com/sarchlab/akita/v3/mem/cache/writeback"
	"github.com/sarchlab/akita/v3/mem/dram"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm/gmmu"
	"github.com/sarchlab/akita/v3/mem/vm/tlb"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/timing/cp"
	"github.com/sarchlab/mgpusim/v3/timing/pagemigrationcontroller"
	"github.com/sarchlab/mgpusim/v3/timing/rdma"
)

func (b *R9NanoGPUBuilder) buildSAs() {
	saBuilder := makeShaderArrayBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithGPUID(b.gpuID).
		WithLog2CachelineSize(b.log2CacheLineSize).
		WithLog2PageSize(b.log2PageSize).
		WithNumCU(b.numCUPerShaderArray)

	if b.enableISADebugging {
		saBuilder = saBuilder.WithIsaDebugging()
	}

	if b.enableVisTracing {
		saBuilder = saBuilder.WithVisTracer(b.visTracer)
	}

	if b.enableMemTracing {
		saBuilder = saBuilder.WithMemTracer(b.memTracer)
	}

	for i := 0; i < b.numShaderArray; i++ {
		saName := fmt.Sprintf("%s.SA[%d]", b.gpuName, i)
		b.buildSA(saBuilder, saName)
	}
}

func (b *R9NanoGPUBuilder) buildL2Caches() {
	byteSize := b.l2CacheSize / uint64(b.numMemoryBank)
	l2Builder := writeback.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(16).
		WithByteSize(byteSize).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(16)

	for i := 0; i < b.numMemoryBank; i++ {
		cacheName := fmt.Sprintf("%s.L2[%d]", b.gpuName, i)
		l2 := l2Builder.WithInterleaving(
			1<<(b.log2MemoryBankInterleavingSize-b.log2CacheLineSize),
			b.numMemoryBank,
			i,
		).Build(cacheName)
		b.l2Caches = append(b.l2Caches, l2)
		b.gpu.L2Caches = append(b.gpu.L2Caches, l2)

		if b.enableVisTracing {
			tracing.CollectTrace(l2, b.visTracer)
		}

		if b.enableMemTracing {
			tracing.CollectTrace(l2, b.memTracer)
		}

		if b.monitor != nil {
			b.monitor.RegisterComponent(l2)
		}

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(l2)
		}
	}
}

func (b *R9NanoGPUBuilder) buildGMMUCache() {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumWays(8).
		WithNumSets(16).
		WithNumMSHREntry(32).
		WithNumReqPerCycle(32).
		WithPageSize(1 << b.log2PageSize).
		WithLowModule(b.gmmu.GetPortByName("Top"))

	gmmuCache := builder.Build(fmt.Sprintf("%s.GMMUCache", b.gpuName))
	b.gmmuCache = gmmuCache
	b.gpu.GMMUCache = append(b.gpu.GMMUCache, gmmuCache)
	// b.gpu.L2TLBs = append(b.gpu.L2TLBs, l2TLB)

	if b.enableVisTracing {
		tracing.CollectTrace(b.gmmuCache, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.gmmuCache)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(b.gmmuCache)
	}
}

func (b *R9NanoGPUBuilder) buildGMMU() {
	gmmu := gmmu.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize).
		WithMaxNumReqInFlight(4096).
		WithPageTable(b.pageTable).
		WithPageWalkingLatency(100).
		WithLowModule(b.mmu.GetPortByName("Top")).
		WithIsPrediction(true).
		Build(fmt.Sprintf("%s.GMMU", b.gpuName))

	b.gmmu = gmmu
	b.gpu.GMMUEngine = b.gmmu

	if b.enableVisTracing {
		tracing.CollectTrace(b.gmmu, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.gmmu)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(b.gmmu)
	}

}

func (b *R9NanoGPUBuilder) buildDRAMControllers() {
	memCtrlBuilder := b.createDramControllerBuilder()

	for i := 0; i < b.numMemoryBank; i++ {
		dramName := fmt.Sprintf("%s.DRAM[%d]", b.gpuName, i)
		dram := memCtrlBuilder.Build(dramName)
		// dram := idealmemcontroller.New(
		// 	fmt.Sprintf("%s.DRAM[%d]", b.gpuName, i),
		// 	b.engine, 512*mem.MB)
		b.drams = append(b.drams, dram)
		b.gpu.MemControllers = append(b.gpu.MemControllers, dram)

		if b.enableMemTracing {
			tracing.CollectTrace(dram, b.memTracer)
		}

		if b.monitor != nil {
			b.monitor.RegisterComponent(dram)
		}

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(dram)
		}
	}
}

func (b *R9NanoGPUBuilder) buildSA(
	saBuilder shaderArrayBuilder,
	saName string,
) {
	sa := saBuilder.Build(saName)

	b.populateCUs(&sa)
	b.populateROBs(&sa)
	b.populateTLBs(&sa)
	b.populateL1VAddressTranslators(&sa)
	b.populateL1Vs(&sa)
	b.populateScalerMemoryHierarchy(&sa)
	b.populateInstMemoryHierarchy(&sa)
}

func (b *R9NanoGPUBuilder) buildRDMAEngine() {
	name := fmt.Sprintf("%s.RDMA", b.gpuName)
	b.rdmaEngine = rdma.MakeBuilder().
		WithEngine(b.engine).
		WithBufferSize(1024).
		WithFreq(b.freq).
		WithLocalModules(b.lowModuleFinderForL1).
		WithRemoteModules(nil).
		Build(name)
	b.gpu.RDMAEngine = b.rdmaEngine
	if b.monitor != nil {
		b.monitor.RegisterComponent(b.rdmaEngine)
	}

	if b.enableVisTracing {
		tracing.CollectTrace(b.rdmaEngine, b.visTracer)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(b.rdmaEngine)
	}
}

func (b *R9NanoGPUBuilder) buildPageMigrationController() {
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

	if b.enableVisTracing {
		tracing.CollectTrace(b.pageMigrationController, b.visTracer)
	}
}

func (b *R9NanoGPUBuilder) buildDMAEngine() {
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

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(b.dmaEngine)
	}
}

func (b *R9NanoGPUBuilder) buildCP() {
	builder := cp.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithMonitor(b.monitor).
		WithPerfAnalyzer(b.perfAnalyzer)

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

func (b *R9NanoGPUBuilder) buildL2TLB() {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumWays(16).
		WithNumSets(32).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(32).
		WithPageSize(1 << b.log2PageSize).
		WithLowModule(b.gmmuCache.GetPortByName("Top"))

	l2TLB := builder.Build(fmt.Sprintf("%s.L2TLB", b.gpuName))
	b.l2TLBs = append(b.l2TLBs, l2TLB)
	b.gpu.L2TLBs = append(b.gpu.L2TLBs, l2TLB)

	if b.enableVisTracing {
		tracing.CollectTrace(l2TLB, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(l2TLB)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(l2TLB)
	}
}

func (b *R9NanoGPUBuilder) createDramControllerBuilder() dram.Builder {
	memBankSize := b.dramSize / uint64(b.numMemoryBank)
	if 4*mem.GB%uint64(b.numMemoryBank) != 0 {
		panic("GPU memory size is not a multiple of the number of memory banks")
	}

	dramCol := 64
	dramRow := 16384
	dramDeviceWidth := 128 * 2
	dramBankSize := dramCol * dramRow * dramDeviceWidth
	dramBank := 4
	dramBankGroup := 4
	dramBusWidth := 256
	dramDevicePerRank := dramBusWidth / dramDeviceWidth
	dramRankSize := dramBankSize * dramDevicePerRank * dramBank
	dramRank := int(memBankSize * 8 / uint64(dramRankSize))
	if dramRank == 0 {
		panic("DRAMRank is 0")
	}

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

	if b.globalStorage != nil {
		memCtrlBuilder = memCtrlBuilder.WithGlobalStorage(b.globalStorage)
	}

	return memCtrlBuilder
}
