package gpuArch

import (
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/sim"
)

func (b *R9NanoGPUBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	conn := sim.NewDirectConnection(
		port1.Name()+"-"+port2.Name(),
		b.engine, b.freq,
	)
	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
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
	b.internalConn.PlugIn(b.cp.ToRDMA, 128)
	b.internalConn.PlugIn(b.cp.ToPMC, 1024)

	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.internalConn.PlugIn(b.cp.RDMA, 128)

	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.internalConn.PlugIn(b.dmaEngine.ToCP, 1)

	pmcControlPort := b.pageMigrationController.GetPortByName("Control")
	b.cp.PMC = pmcControlPort
	b.internalConn.PlugIn(pmcControlPort, 1)
}

func (b *R9NanoGPUBuilder) connectCPWithCUs() {
	for _, cu := range b.cus {
		b.cp.RegisterCU(cu)
		b.internalConn.PlugIn(cu.ToACE, 1)
		b.internalConn.PlugIn(cu.ToCP, 1)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithAddressTranslators() {
	for _, at := range b.l1vAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, at := range b.l1sAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, at := range b.l1iAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, rob := range b.l1vReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, rob := range b.l1iReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, rob := range b.l1sReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithTLBs() {
	for _, tlb := range b.l2TLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, tlb := range b.l1vTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, tlb := range b.l1sTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, tlb := range b.l1iTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}
}

func (b *R9NanoGPUBuilder) connectCPWithCaches() {
	for _, c := range b.l1iCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1ICaches = append(b.cp.L1ICaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, c := range b.l1vCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1VCaches = append(b.cp.L1VCaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, c := range b.l1sCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1SCaches = append(b.cp.L1SCaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, c := range b.l2Caches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L2Caches = append(b.cp.L2Caches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}
}

func (b *R9NanoGPUBuilder) connectL1ToL2() {
	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)
	lowModuleFinder.ModuleForOtherAddresses = b.rdmaEngine.ToL1
	lowModuleFinder.UseAddressSpaceLimitation = true
	lowModuleFinder.LowAddress = b.memAddrOffset
	lowModuleFinder.HighAddress = b.memAddrOffset + 8*mem.GB

	l1ToL2Conn := sim.NewDirectConnection(b.gpuName+".L1toL2",
		b.engine, b.freq)

	b.rdmaEngine.SetLocalModuleFinder(lowModuleFinder)
	l1ToL2Conn.PlugIn(b.rdmaEngine.ToL1, 1024)
	l1ToL2Conn.PlugIn(b.rdmaEngine.ToL2, 1024)

	for _, l2 := range b.l2Caches {
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			l2.GetPortByName("Top"))
		l1ToL2Conn.PlugIn(l2.GetPortByName("Top"), 64)
	}

	for _, l1v := range b.l1vCaches {
		l1v.SetLowModuleFinder(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1v.GetPortByName("Bottom"), 16)
	}

	for _, l1s := range b.l1sCaches {
		l1s.SetLowModuleFinder(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1s.GetPortByName("Bottom"), 16)
	}

	for _, l1iAT := range b.l1iAddrTrans {
		l1iAT.SetLowModuleFinder(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1iAT.GetPortByName("Bottom"), 16)
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

func (b *R9NanoGPUBuilder) connectL2AndDRAM() {
	b.l2ToDramConnection = sim.NewDirectConnection(
		b.gpuName+".L2toDRAM", b.engine, b.freq)

	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)

	for i, l2 := range b.l2Caches {
		b.l2ToDramConnection.PlugIn(l2.GetPortByName("Bottom"), 64)
		l2.SetLowModuleFinder(&mem.SingleLowModuleFinder{
			LowModule: b.drams[i].GetPortByName("Top"),
		})
	}

	for _, dram := range b.drams {
		b.l2ToDramConnection.PlugIn(dram.GetPortByName("Top"), 64)
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			dram.GetPortByName("Top"))
	}

	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
	b.l2ToDramConnection.PlugIn(b.dmaEngine.ToMem, 64)

	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
	b.l2ToDramConnection.PlugIn(
		b.pageMigrationController.GetPortByName("LocalMem"), 16)
}

func (b *R9NanoGPUBuilder) connectL2TLBToGMMUCache() {
	conn := sim.NewDirectConnection(
		b.gpuName+".L2TLBtoGMMUCache",
		b.engine, b.freq,
	)
	conn.PlugIn(b.gmmuCache.GetPortByName("Top"), 64)

	for _, l2TLB := range b.l2TLBs {
		conn.PlugIn(l2TLB.GetPortByName("Bottom"), 64)
	}
}

func (b *R9NanoGPUBuilder) connectGMMUCachetoGMMU() {
	conn := sim.NewDirectConnection(
		b.gpuName+".GMMUCacheToGMMU",
		b.engine, b.freq,
	)
	conn.PlugIn(b.gmmu.GetPortByName("Top"), 64)
	conn.PlugIn(b.gmmuCache.GetPortByName("Bottom"), 64)
}
