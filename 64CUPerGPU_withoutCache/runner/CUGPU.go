package runner

import (
	"fmt"

	"github.com/sarchlab/mgpusim/v3/timing/rdma"
	rob2 "github.com/sarchlab/mgpusim/v3/timing/rob"

	"github.com/sarchlab/akita/v3/mem/idealmemcontroller"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm/addresstranslator"
	"github.com/sarchlab/akita/v3/mem/vm/mmu"
	"github.com/sarchlab/akita/v3/mem/vm/tlb"
	"github.com/sarchlab/akita/v3/monitoring"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/timing/cp"
	"github.com/sarchlab/mgpusim/v3/timing/cu"
	"github.com/sarchlab/mgpusim/v3/timing/pagemigrationcontroller"
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
	cus                     []*cu.ComputeUnit
	l1vReorderBuffers       []*rob2.ReorderBuffer
	l1iReorderBuffers       []*rob2.ReorderBuffer
	l1sReorderBuffers       []*rob2.ReorderBuffer
	l1vAddrTrans            []*addresstranslator.AddressTranslator
	l1sAddrTrans            []*addresstranslator.AddressTranslator
	l1iAddrTrans            []*addresstranslator.AddressTranslator
	l1vTLBs                 []*tlb.TLB
	l1sTLBs                 []*tlb.TLB
	l1iTLBs                 []*tlb.TLB
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
		numShader:                      16,
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
		withLog2PageSize(b.log2PageSize).
		withLog2CachelineSize(b.log2CacheLineSize)

	for i := 0; i < b.numShader; i++ {
		name := fmt.Sprintf("%s.Shader[%v]", b.gpuName, i)
		shader := saBuilder.Build(name)

		b.populateCUs(&shader)
		b.populateROBs(&shader)
		b.populateTLBs(&shader)
		b.populateL1VAddressTranslators(&shader)
		b.populateScalerMemoryHierarchys(&shader)
		b.populateInstMemoryHierarchys(&shader)

	}

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

func (b *R9NanoGPUBuilder) populateCUs(s *shader) {
	for _, cu := range s.computeUnits {
		b.cus = append(b.cus, cu)
		b.gpu.CUs = append(b.gpu.CUs, cu)

		if b.monitor != nil {
			b.monitor.RegisterComponent(cu)
		}
	}
}

func (b *R9NanoGPUBuilder) populateROBs(s *shader) {
	for _, l1vRoB := range s.vectorRoBs {
		b.l1vReorderBuffers = append(b.l1vReorderBuffers, l1vRoB)

		if b.monitor != nil {
			b.monitor.RegisterComponent(l1vRoB)
		}
	}
}

func (b *R9NanoGPUBuilder) populateTLBs(s *shader) {
	for _, l1vTLB := range s.vectorTLBs {
		b.l1vTLBs = append(b.l1vTLBs, l1vTLB)
		b.gpu.L1vTLBs = append(b.gpu.L1vTLBs, l1vTLB)

		if b.monitor != nil {
			b.monitor.RegisterComponent(l1vTLB)
		}
	}
}

func (b *R9NanoGPUBuilder) populateL1VAddressTranslators(s *shader) {
	for _, l1vAddrTran := range s.vectorAddressTranslators {
		b.l1vAddrTrans = append(b.l1vAddrTrans, l1vAddrTran)

		if b.monitor != nil {
			b.monitor.RegisterComponent(l1vAddrTran)
		}
	}
}

func (b *R9NanoGPUBuilder) populateScalerMemoryHierarchys(s *shader) {
	b.l1sAddrTrans = append(b.l1sAddrTrans, s.scalarAddressTranslator)
	b.l1sReorderBuffers = append(b.l1sReorderBuffers, s.scalarRoB)
	b.l1sTLBs = append(b.l1sTLBs, s.scalarTLB)
	b.gpu.L1STLBs = append(b.gpu.L1STLBs, s.scalarTLB)

	if b.monitor != nil {
		b.monitor.RegisterComponent(s.scalarAddressTranslator)
		b.monitor.RegisterComponent(s.scalarRoB)
		b.monitor.RegisterComponent(s.scalarTLB)
	}
}

func (b *R9NanoGPUBuilder) populateInstMemoryHierarchys(s *shader) {
	b.l1iAddrTrans = append(b.l1iAddrTrans, s.instructionAddressTranslator)
	b.l1iReorderBuffers = append(b.l1iReorderBuffers, s.instructionRoB)
	b.l1iTLBs = append(b.l1iTLBs, s.instructionTLB)
	b.gpu.L1ITLBs = append(b.gpu.L1ITLBs, s.instructionTLB)

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
	for _, cu := range b.cus {
		b.cp.RegisterCU(cu)
		b.internalConn.PlugIn(cu.ToACE, 1)
		b.internalConn.PlugIn(cu.ToCP, 1)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithAddressTranslators() {

	for _, vat := range b.l1vAddrTrans {
		vATCtrlPort := vat.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, vATCtrlPort)
		b.internalConn.PlugIn(vATCtrlPort, 1)
	}

	for _, sat := range b.l1sAddrTrans {
		sATCtrlPort := sat.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, sATCtrlPort)
		b.internalConn.PlugIn(sATCtrlPort, 1)
	}

	for _, iat := range b.l1iAddrTrans {
		iATCtrlPort := iat.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, iATCtrlPort)
		b.internalConn.PlugIn(iATCtrlPort, 1)
	}

	for _, vROB := range b.l1vReorderBuffers {
		vROBctrlPort := vROB.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, vROBctrlPort)
		b.internalConn.PlugIn(vROBctrlPort, 1)
	}

	for _, iROB := range b.l1iReorderBuffers {
		iROBctrlPort := iROB.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, iROBctrlPort)
		b.internalConn.PlugIn(iROBctrlPort, 1)
	}

	for _, sROB := range b.l1sReorderBuffers {
		sROBctrlPort := sROB.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, sROBctrlPort)
		b.internalConn.PlugIn(sROBctrlPort, 1)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithTLBs() {
	for _, tlb := range b.l2TLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, vtlb := range b.l1vTLBs {
		vCtrlPort := vtlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, vCtrlPort)
		b.internalConn.PlugIn(vCtrlPort, 1)
	}

	for _, stlb := range b.l1sTLBs {
		sCtrlPort := stlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, sCtrlPort)
		b.internalConn.PlugIn(sCtrlPort, 1)
	}

	for _, itlb := range b.l1iTLBs {
		ictrlPort := itlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ictrlPort)
		b.internalConn.PlugIn(ictrlPort, 1)
	}
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

	for _, l1vTLB := range b.l1vTLBs {
		l1vTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
		tlbConn.PlugIn(l1vTLB.GetPortByName("Bottom"), 16)
	}

	for _, l1iTLB := range b.l1iTLBs {
		l1iTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
		tlbConn.PlugIn(l1iTLB.GetPortByName("Bottom"), 16)
	}

	for _, l1sTLB := range b.l1sTLBs {
		l1sTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
		tlbConn.PlugIn(l1sTLB.GetPortByName("Bottom"), 16)
	}
}

func (b *R9NanoGPUBuilder) connectATToDRAM() {
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

	for _, vAT := range b.l1vAddrTrans {
		vAT.SetLowModuleFinder(lowModuleFinder)
		b.atToDramConnection.PlugIn(vAT.GetPortByName("Bottom"), 16)
	}

	for _, iAT := range b.l1iAddrTrans {
		iAT.SetLowModuleFinder(lowModuleFinder)
		b.atToDramConnection.PlugIn(iAT.GetPortByName("Bottom"), 16)
	}

	for _, sAT := range b.l1sAddrTrans {
		sAT.SetLowModuleFinder(lowModuleFinder)
		b.atToDramConnection.PlugIn(sAT.GetPortByName("Bottom"), 16)
	}

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
