package runner

import (
	"fmt"

	"gitlab.com/akita/mgpusim/v3/timing/rdma"
	rob2 "gitlab.com/akita/mgpusim/v3/timing/rob"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/idealmemcontroller"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/timing/cp"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/pagemigrationcontroller"
)

type R9NanoGPUBuilder struct {
	engine                         sim.Engine
	freq                           sim.Freq
	memAddrOffset                  uint64
	mmu                            *mmu.MMU
	numShader                      int
	numMemoryBank                  int
	dramSize                       uint64
	log2PageSize                   uint64
	log2MemoryBankInterleavingSize uint64
	log2CacheLineSize              uint64

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
	cu                      *cu.ComputeUnit
	l1vReorderBuffer        *rob2.ReorderBuffer
	l1iReorderBuffer        *rob2.ReorderBuffer
	l1sReorderBuffer        *rob2.ReorderBuffer
	l1vAddrTran             *addresstranslator.AddressTranslator
	l1sAddrTran             *addresstranslator.AddressTranslator
	l1iAddrTran             *addresstranslator.AddressTranslator
	l1vTLB                  *tlb.TLB
	l1sTLB                  *tlb.TLB
	l1iTLB                  *tlb.TLB
	l2TLBs                  []*tlb.TLB
	drams                   []*idealmemcontroller.Comp
	lowModuleFinderForL1    *mem.InterleavedLowModuleFinder
	lowModuleFinderForPMC   *mem.InterleavedLowModuleFinder
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Engine
	pageMigrationController *pagemigrationcontroller.PageMigrationController
	globalStorage           *mem.Storage

	internalConn           *sim.DirectConnection
	l1TLBToL2TLBConnection *sim.DirectConnection
	atToDramConnection     *sim.DirectConnection
}

func MakeR9NanoGPUBuilder() R9NanoGPUBuilder {
	b := R9NanoGPUBuilder{
		freq:                           1 * sim.GHz,
		numShader:                      1,
		numMemoryBank:                  16,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 12,
		dramSize:                       4 * mem.GB,
	}
	return b
}

func (b R9NanoGPUBuilder) Build(name string, id uint64) *GPU {
	b.createGPU(name, id)
	fmt.Sprintf("build GPU %s \n", b.gpuName)
	b.buildShader()
	b.buildDRAMControllers()
	b.buildCP()
	b.buildL2TLB()

	b.connectCP()
	b.connectATToDRAM()
	b.connectL1TLBToL2TLB()

	b.populateExternalPorts()

	return b.gpu
}

func (b *R9NanoGPUBuilder) createGPU(name string, id uint64) {
	b.gpuName = name

	b.gpu = &GPU{}
	b.gpu.Domain = sim.NewDomain(b.gpuName)
	b.gpuID = id
}

func (b *R9NanoGPUBuilder) buildShader() {
	saBuilder := makeShaderBuilder().
		withEngine(b.engine).
		withFreq(b.freq).
		withGPUID(b.gpuID).
		withLog2PageSize(b.log2PageSize).withLog2CachelineSize(b.log2CacheLineSize)

	name := fmt.Sprintf("%s.Shader", b.gpuName)
	shader := saBuilder.Build(name)

	b.populateCU(&shader)
	b.populateROB(&shader)
	b.populateTLB(&shader)
	b.populateL1VAddressTranslator(&shader)
	b.populateScalerMemoryHierarchy(&shader)
	b.populateInstMemoryHierarchy(&shader)

	if b.enableISADebugging {
		saBuilder = saBuilder.withIsaDebugging()
	}

	if b.enableVisTracing {
		saBuilder = saBuilder.withVisTracer(b.visTracer)
	}

	if b.enableMemTracing {
		saBuilder = saBuilder.withMemTracer(b.memTracer)
	}
}

func (b *R9NanoGPUBuilder) populateCU(s *shader) {
	b.cu = s.computeUnit
	b.gpu.CU = s.computeUnit

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.computeUnit)
	}
}

func (b *R9NanoGPUBuilder) populateROB(s *shader) {
	b.l1vReorderBuffer = s.vectorRoB

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.vectorRoB)
	}
}

func (b *R9NanoGPUBuilder) populateTLB(s *shader) {
	b.l1vTLB = s.vectorTLB

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.vectorTLB)
	}
}

func (b *R9NanoGPUBuilder) populateL1VAddressTranslator(s *shader) {
	b.l1vAddrTran = s.vectorAddressTranslator

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.vectorAddressTranslator)
	}
}

func (b *R9NanoGPUBuilder) populateScalerMemoryHierarchy(s *shader) {
	b.l1sAddrTran = s.scalarAddressTranslator
	b.l1sReorderBuffer = s.scalarRoB
	b.l1sTLB = s.scalarTLB
	b.gpu.L1STLB = s.scalarTLB

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.scalarAddressTranslator)
		b.monitor.RegisterComponent(s.scalarRoB)
		b.monitor.RegisterComponent(s.scalarTLB)
	}
}

func (b *R9NanoGPUBuilder) populateInstMemoryHierarchy(s *shader) {
	b.l1iAddrTran = s.instructionAddressTranslator
	b.l1iReorderBuffer = s.instructionRoB
	b.l1iTLB = s.instructionTLB
	b.gpu.L1ITLB = s.instructionTLB

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.instructionAddressTranslator)
		b.monitor.RegisterComponent(s.instructionRoB)
		b.monitor.RegisterComponent(s.instructionTLB)
	}
}

func (b *R9NanoGPUBuilder) buildDRAMControllers() {

	for i := 0; i < b.numMemoryBank; i++ {
		dram := idealmemcontroller.New(
			fmt.Sprintf("%s.DRAM[%d]", b.gpuName, i), b.engine, 512*mem.MB)
		dram.Latency = 1
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

		if b.globalStorage != nil {
			dram.Storage = b.globalStorage
		}
	}
}

func (b *R9NanoGPUBuilder) buildCP() {
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
}

func (b *R9NanoGPUBuilder) buildRDMAEngine() {
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

	if b.enableVisTracing {
		tracing.CollectTrace(b.rdmaEngine, b.visTracer)
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
}

func (b *R9NanoGPUBuilder) buildL2TLB() {
	numWays := 64
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumWays(numWays).
		WithNumSets(int(b.dramSize / (1 << b.log2PageSize) / uint64(numWays))).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(1024).
		WithPageSize(1 << b.log2PageSize).
		WithLowModule(b.mmu.GetPortByName("Top"))

	l2TLB := builder.Build(fmt.Sprintf("%s.L2TLB", b.gpuName))
	b.l2TLBs = append(b.l2TLBs, l2TLB)
	b.gpu.L2TLBs = append(b.gpu.L2TLBs, l2TLB)

	if b.enableVisTracing {
		tracing.CollectTrace(l2TLB, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(l2TLB)
	}
}

func (b *R9NanoGPUBuilder) connectCP() {
	b.internalConn = sim.NewDirectConnection(
		b.gpuName+".InternalConn", b.engine, b.freq)

	b.internalConn.PlugIn(b.cp.ToDriver, 1)
	b.internalConn.PlugIn(b.cp.ToDMA, 128)
	b.internalConn.PlugIn(b.cp.ToCaches, 128)
	b.internalConn.PlugIn(b.cp.ToCUs, 128)
	b.internalConn.PlugIn(b.cp.ToTLBs, 128)
	b.internalConn.PlugIn(b.cp.ToAddressTranslators, 128)
	b.internalConn.PlugIn(b.cp.ToRDMA, 4)
	b.internalConn.PlugIn(b.cp.ToPMC, 4)

	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.internalConn.PlugIn(b.cp.RDMA, 1)

	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.internalConn.PlugIn(b.dmaEngine.ToCP, 1)

	pmcControlPort := b.pageMigrationController.GetPortByName("Control")
	b.cp.PMC = pmcControlPort
	b.internalConn.PlugIn(pmcControlPort, 1)

	b.connectCPWithCUs()
	b.connectCPWithAddressTranslators()
	b.connectCPWithTLBs()
	// b.connectCPWithCaches()
}

func (b *R9NanoGPUBuilder) connectCPWithCUs() {
	b.cp.RegisterCU(b.cu)
	b.internalConn.PlugIn(b.cu.ToACE, 1)
	b.internalConn.PlugIn(b.cu.ToCP, 1)
}

func (b *R9NanoGPUBuilder) connectCPWithAddressTranslators() {

	vat := b.l1vAddrTran
	vATCtrlPort := vat.GetPortByName("Control")
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, vATCtrlPort)
	b.internalConn.PlugIn(vATCtrlPort, 1)

	sat := b.l1sAddrTran
	sATCtrlPort := sat.GetPortByName("Control")
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, sATCtrlPort)
	b.internalConn.PlugIn(sATCtrlPort, 1)

	iat := b.l1iAddrTran
	iATCtrlPort := iat.GetPortByName("Control")
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, iATCtrlPort)
	b.internalConn.PlugIn(iATCtrlPort, 1)

	vROB := b.l1vReorderBuffer
	vROBctrlPort := vROB.GetPortByName("Control")
	b.cp.AddressTranslators = append(
		b.cp.AddressTranslators, vROBctrlPort)
	b.internalConn.PlugIn(vROBctrlPort, 1)

	iROB := b.l1iReorderBuffer
	iROBctrlPort := iROB.GetPortByName("Control")
	b.cp.AddressTranslators = append(
		b.cp.AddressTranslators, iROBctrlPort)
	b.internalConn.PlugIn(iROBctrlPort, 1)

	sROB := b.l1vReorderBuffer
	sROBctrlPort := sROB.GetPortByName("Control")
	b.cp.AddressTranslators = append(
		b.cp.AddressTranslators, sROBctrlPort)
	b.internalConn.PlugIn(sROBctrlPort, 1)
}

func (b *R9NanoGPUBuilder) connectCPWithTLBs() {
	for _, tlb := range b.l2TLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	vtlb := b.l1vTLB
	vCtrlPort := vtlb.GetPortByName("Control")
	b.cp.TLBs = append(b.cp.TLBs, vCtrlPort)
	b.internalConn.PlugIn(vCtrlPort, 1)

	stlb := b.l1sTLB
	sCtrlPort := stlb.GetPortByName("Control")
	b.cp.TLBs = append(b.cp.TLBs, sCtrlPort)
	b.internalConn.PlugIn(sCtrlPort, 1)

	itlb := b.l1iTLB
	ictrlPort := itlb.GetPortByName("Control")
	b.cp.TLBs = append(b.cp.TLBs, ictrlPort)
	b.internalConn.PlugIn(ictrlPort, 1)
}

func (b *R9NanoGPUBuilder) populateExternalPorts() {
	b.gpu.Domain.AddPort("CommandProcessor", b.cp.ToDriver)
	b.gpu.Domain.AddPort("RDMA", b.rdmaEngine.ToOutside)
	b.gpu.Domain.AddPort("PageMigrationController",
		b.pageMigrationController.GetPortByName("Remote"))

	for i, tlb := range b.l2TLBs {
		name := fmt.Sprintf("Translation[%d]", i)
		b.gpu.Domain.AddPort(name, tlb.GetPortByName("Bottom"))
	}

}

func (b *R9NanoGPUBuilder) connectL1TLBToL2TLB() {
	tlbConn := sim.NewDirectConnection(b.gpuName+".L1TLBtoL2TLB",
		b.engine, b.freq)

	tlbConn.PlugIn(b.l2TLBs[0].GetPortByName("Top"), 64)

	l1vTLB := b.l1vTLB
	l1vTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
	tlbConn.PlugIn(l1vTLB.GetPortByName("Bottom"), 16)

	l1iTLB := b.l1iTLB
	l1iTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
	tlbConn.PlugIn(l1iTLB.GetPortByName("Bottom"), 16)

	l1sTLB := b.l1sTLB
	l1sTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
	tlbConn.PlugIn(l1sTLB.GetPortByName("Bottom"), 16)
}

func (b *R9NanoGPUBuilder) connectATToDRAM() {
	iAT := b.l1iAddrTran
	vAT := b.l1vAddrTran
	sAT := b.l1sAddrTran

	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)
	lowModuleFinder.ModuleForOtherAddresses = b.rdmaEngine.ToL1
	lowModuleFinder.UseAddressSpaceLimitation = true
	lowModuleFinder.LowAddress = b.memAddrOffset
	lowModuleFinder.HighAddress = b.memAddrOffset + 4*mem.GB

	connName := fmt.Sprintf("%s.ATToDRAM", b.gpuName)
	b.atToDramConnection = sim.NewDirectConnection(connName, b.engine, b.freq)
	b.atToDramConnection.PlugIn(b.rdmaEngine.ToL1, 64)
	b.atToDramConnection.PlugIn(b.rdmaEngine.ToL2, 64)

	iAT.SetLowModuleFinder(lowModuleFinder)
	vAT.SetLowModuleFinder(lowModuleFinder)
	sAT.SetLowModuleFinder(lowModuleFinder)

	b.atToDramConnection.PlugIn(iAT.GetPortByName("Bottom"), 16)
	b.atToDramConnection.PlugIn(vAT.GetPortByName("Bottom"), 16)
	b.atToDramConnection.PlugIn(sAT.GetPortByName("Bottom"), 16)

	for _, dram := range b.drams {
		b.atToDramConnection.PlugIn(dram.GetPortByName("Top"), 64)
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			dram.GetPortByName("Top"))
	}

	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
	b.atToDramConnection.PlugIn(b.dmaEngine.ToMem, 64)

	b.rdmaEngine.SetLocalModuleFinder(lowModuleFinder)

	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
	b.atToDramConnection.PlugIn(
		b.pageMigrationController.GetPortByName("LocalMem"), 16)

}

// WithEngine sets the engine that the GPU use.
func (b R9NanoGPUBuilder) WithEngine(engine sim.Engine) R9NanoGPUBuilder {
	b.engine = engine
	return b
}

// WithFreq sets the frequency that the GPU works at.
func (b R9NanoGPUBuilder) WithFreq(freq sim.Freq) R9NanoGPUBuilder {
	b.freq = freq
	return b
}

// WithMemAddrOffset sets the address of the first byte of the GPU to build.
func (b R9NanoGPUBuilder) WithMemAddrOffset(
	offset uint64,
) R9NanoGPUBuilder {
	b.memAddrOffset = offset
	return b
}

// WithMMU sets the MMU component that provides the address translation service
// for the GPU.
func (b R9NanoGPUBuilder) WithMMU(mmu *mmu.MMU) R9NanoGPUBuilder {
	b.mmu = mmu
	return b
}

// WithNumMemoryBank sets the number of L2 cache modules and number of memory
// controllers in each GPU.
func (b R9NanoGPUBuilder) WithNumMemoryBank(n int) R9NanoGPUBuilder {
	b.numMemoryBank = n
	return b
}

// WithLog2MemoryBankInterleavingSize sets the number of consecutive bytes that
// are guaranteed to be on a memory bank.
func (b R9NanoGPUBuilder) WithLog2MemoryBankInterleavingSize(
	n uint64,
) R9NanoGPUBuilder {
	b.log2MemoryBankInterleavingSize = n
	return b
}

// WithVisTracer applies a tracer to trace all the tasks of all the GPU
// components
func (b R9NanoGPUBuilder) WithVisTracer(t tracing.Tracer) R9NanoGPUBuilder {
	b.enableVisTracing = true
	b.visTracer = t
	return b
}

// WithMemTracer applies a tracer to trace the memory transactions.
func (b R9NanoGPUBuilder) WithMemTracer(t tracing.Tracer) R9NanoGPUBuilder {
	b.enableMemTracing = true
	b.memTracer = t
	return b
}

// WithISADebugging enables the GPU to dump instruction execution information.
func (b R9NanoGPUBuilder) WithISADebugging() R9NanoGPUBuilder {
	b.enableISADebugging = true
	return b
}

// WithLog2PageSize sets the page size with the power of 2.
func (b R9NanoGPUBuilder) WithLog2PageSize(log2PageSize uint64) R9NanoGPUBuilder {
	b.log2PageSize = log2PageSize
	return b
}

// WithMonitor sets the monitor to use.
func (b R9NanoGPUBuilder) WithMonitor(m *monitoring.Monitor) R9NanoGPUBuilder {
	b.monitor = m
	return b
}

// WithDRAMSize sets the size of DRAMs in the GPU.
func (b R9NanoGPUBuilder) WithDRAMSize(size uint64) R9NanoGPUBuilder {
	b.dramSize = size
	return b
}

// WithGlobalStorage lets the GPU to build to use the externally provided
// storage.
func (b R9NanoGPUBuilder) WithGlobalStorage(
	storage *mem.Storage,
) R9NanoGPUBuilder {
	b.globalStorage = storage
	return b
}

func (b R9NanoGPUBuilder) withLog2CachelineSize(size uint64) R9NanoGPUBuilder {
	b.log2CacheLineSize = size
	return b
}