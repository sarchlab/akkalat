package gpuArch

import (
	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/sim/directconnection"
)

func (b *R9NanoGPUBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(port1.Name() + "-" + port2.Name())
	conn.PlugIn(port1)
	conn.PlugIn(port2)
}

func (b *R9NanoGPUBuilder) connectCP() {
	b.internalConn = directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.gpuName + ".InternalConn")

	b.internalConn.PlugIn(b.cp.ToDriver)
	b.internalConn.PlugIn(b.cp.ToDMA)
	b.internalConn.PlugIn(b.cp.ToCaches)
	b.internalConn.PlugIn(b.cp.ToCUs)
	b.internalConn.PlugIn(b.cp.ToTLBs)
	b.internalConn.PlugIn(b.cp.ToAddressTranslators)
	b.internalConn.PlugIn(b.cp.ToRDMA)
	b.internalConn.PlugIn(b.cp.ToPMC)

	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.internalConn.PlugIn(b.cp.RDMA)

	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.internalConn.PlugIn(b.dmaEngine.ToCP)

	pmcControlPort := b.pageMigrationController.GetPortByName("Control")
	b.cp.PMC = pmcControlPort
	b.internalConn.PlugIn(pmcControlPort)
}

func (b *R9NanoGPUBuilder) connectCPWithCUs() {
	for _, cu := range b.cus {
		b.cp.RegisterCU(cu)
		b.internalConn.PlugIn(cu.ToACE)
		b.internalConn.PlugIn(cu.ToCP)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithAddressTranslators() {
	for _, at := range b.l1vAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, at := range b.l1sAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, at := range b.l1iAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, rob := range b.l1vReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, rob := range b.l1iReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, rob := range b.l1sReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithTLBs() {
	for _, tlb := range b.l2TLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, tlb := range b.l1vTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, tlb := range b.l1sTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, tlb := range b.l1iTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithCaches() {
	for _, c := range b.l1iCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1ICaches = append(b.cp.L1ICaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, c := range b.l1vCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1VCaches = append(b.cp.L1VCaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, c := range b.l1sCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1SCaches = append(b.cp.L1SCaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}

	for _, c := range b.l2Caches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L2Caches = append(b.cp.L2Caches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort)
	}
}

func (b *R9NanoGPUBuilder) connectL1ToL2() {
	lowModuleFinder := mem.NewInterleavedAddressPortMapper(
		1 << b.log2MemoryBankInterleavingSize)
	lowModuleFinder.ModuleForOtherAddresses = b.rdmaEngine.ToL1.AsRemote()
	lowModuleFinder.UseAddressSpaceLimitation = true
	lowModuleFinder.LowAddress = b.memAddrOffset
	lowModuleFinder.HighAddress = b.memAddrOffset + 8*mem.GB

	l1ToL2Conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.gpuName + ".L1toL2")

	b.rdmaEngine.SetLocalModuleFinder(lowModuleFinder)
	l1ToL2Conn.PlugIn(b.rdmaEngine.ToL1)
	l1ToL2Conn.PlugIn(b.rdmaEngine.ToL2)

	for _, l2 := range b.l2Caches {
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			l2.GetPortByName("Top").AsRemote())
		l1ToL2Conn.PlugIn(l2.GetPortByName("Top"))
	}

	for _, l1v := range b.l1vCaches {
		l1v.SetAddressToPortMapper(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1v.GetPortByName("Bottom"))
	}

	for _, l1s := range b.l1sCaches {
		l1s.SetAddressToPortMapper(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1s.GetPortByName("Bottom"))
	}

	for _, l1iAT := range b.l1iAddrTrans {
		l1iAT.SetAddressToPortMapper(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1iAT.GetPortByName("Bottom"))
	}
}

func (b *R9NanoGPUBuilder) connectL1TLBToL2TLB() {
	tlbConn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.gpuName + ".L1TLBtoL2TLB")

	tlbConn.PlugIn(b.l2TLBs[0].GetPortByName("Top"))

	for _, l1vTLB := range b.l1vTLBs {
		l1vTLB.LowModule = b.l2TLBs[0].GetPortByName("Top").AsRemote()
		tlbConn.PlugIn(l1vTLB.GetPortByName("Bottom"))
	}

	for _, l1iTLB := range b.l1iTLBs {
		l1iTLB.LowModule = b.l2TLBs[0].GetPortByName("Top").AsRemote()
		tlbConn.PlugIn(l1iTLB.GetPortByName("Bottom"))
	}

	for _, l1sTLB := range b.l1sTLBs {
		l1sTLB.LowModule = b.l2TLBs[0].GetPortByName("Top").AsRemote()
		tlbConn.PlugIn(l1sTLB.GetPortByName("Bottom"))
	}
}

func (b *R9NanoGPUBuilder) connectL2AndDRAM() {
	b.l2ToDramConnection = directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.gpuName + ".L2toDRAM")

	lowModuleFinder := mem.NewInterleavedAddressPortMapper(
		1 << b.log2MemoryBankInterleavingSize)

	for i, l2 := range b.l2Caches {
		b.l2ToDramConnection.PlugIn(l2.GetPortByName("Bottom"))
		l2.SetAddressToPortMapper(&mem.SinglePortMapper{
			Port: b.drams[i].GetPortByName("Top").AsRemote(),
		})
	}

	for _, dram := range b.drams {
		b.l2ToDramConnection.PlugIn(dram.GetPortByName("Top"))
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			dram.GetPortByName("Top").AsRemote())
	}

	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
	b.l2ToDramConnection.PlugIn(b.dmaEngine.ToMem)

	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
	b.l2ToDramConnection.PlugIn(
		b.pageMigrationController.GetPortByName("LocalMem"))
}

func (b *R9NanoGPUBuilder) connectL2TLBToGMMUCache() {
	conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.gpuName + ".L2TLBtoGMMUCache")
	conn.PlugIn(b.gmmuCache.GetPortByName("Top"))

	for _, l2TLB := range b.l2TLBs {
		conn.PlugIn(l2TLB.GetPortByName("Bottom"))
	}
}

func (b *R9NanoGPUBuilder) connectGMMUCachetoGMMU() {
	conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.gpuName + ".GMMUCacheToGMMU")
	conn.PlugIn(b.gmmu.GetPortByName("Top"))
	conn.PlugIn(b.gmmuCache.GetPortByName("Bottom"))
}
