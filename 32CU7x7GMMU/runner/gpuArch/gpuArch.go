package gpuArch

import (
	rob2 "github.com/sarchlab/mgpusim/v3/timing/rob"

	"github.com/sarchlab/akita/v3/analysis"
	"github.com/sarchlab/akita/v3/mem/cache/writearound"
	"github.com/sarchlab/akita/v3/mem/cache/writeback"
	"github.com/sarchlab/akita/v3/mem/cache/writethrough"
	"github.com/sarchlab/akita/v3/mem/dram"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/mem/vm/addresstranslator"
	"github.com/sarchlab/akita/v3/mem/vm/gmmu"
	"github.com/sarchlab/akita/v3/mem/vm/mmu"
	"github.com/sarchlab/akita/v3/mem/vm/tlb"
	"github.com/sarchlab/akita/v3/monitoring"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/timing/cp"
	"github.com/sarchlab/mgpusim/v3/timing/cu"
	"github.com/sarchlab/mgpusim/v3/timing/pagemigrationcontroller"
	"github.com/sarchlab/mgpusim/v3/timing/rdma"
)

// R9NanoGPUBuilder can build R9 Nano GPUs.
type R9NanoGPUBuilder struct {
	engine                         sim.Engine
	freq                           sim.Freq
	memAddrOffset                  uint64
	mmu                            *mmu.MMU
	numShaderArray                 int
	numCUPerShaderArray            int
	numMemoryBank                  int
	dramSize                       uint64
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
	perfAnalyzer       *analysis.PerfAnalyzer

	gpuName           string
	gpu               *GPU
	gpuID             uint64
	cp                *cp.CommandProcessor
	cus               []*cu.ComputeUnit
	l1vReorderBuffers []*rob2.ReorderBuffer
	l1iReorderBuffers []*rob2.ReorderBuffer
	l1sReorderBuffers []*rob2.ReorderBuffer
	l1vCaches         []*writearound.Cache
	l1sCaches         []*writethrough.Cache
	l1iCaches         []*writethrough.Cache
	l2Caches          []*writeback.Cache
	l1vAddrTrans      []*addresstranslator.AddressTranslator
	l1sAddrTrans      []*addresstranslator.AddressTranslator
	l1iAddrTrans      []*addresstranslator.AddressTranslator
	l1vTLBs           []*tlb.TLB
	l1sTLBs           []*tlb.TLB
	l1iTLBs           []*tlb.TLB
	l2TLBs            []*tlb.TLB
	gmmuCache         *tlb.TLB
	gmmu              *gmmu.GMMU
	drams             []*dram.MemController
	// drams                   []*idealmemcontroller.Comp
	lowModuleFinderForL1    *mem.InterleavedLowModuleFinder
	lowModuleFinderForL2    *mem.InterleavedLowModuleFinder
	lowModuleFinderForPMC   *mem.InterleavedLowModuleFinder
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Comp
	pageMigrationController *pagemigrationcontroller.PageMigrationController
	globalStorage           *mem.Storage
	pageTable               vm.PageTable

	internalConn           *sim.DirectConnection
	l1TLBToL2TLBConnection *sim.DirectConnection
	l1ToL2Connection       *sim.DirectConnection
	l2ToDramConnection     *sim.DirectConnection
}

// MakeR9NanoGPUBuilder provides a GPU builder that can builds the R9Nano GPU.
func MakeR9NanoGPUBuilder() R9NanoGPUBuilder {
	b := R9NanoGPUBuilder{
		freq:                           1 * sim.GHz,
		numShaderArray:                 8,
		numCUPerShaderArray:            4,
		numMemoryBank:                  16,
		log2CacheLineSize:              6,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 12,
		l2CacheSize:                    4 * mem.MB,
		dramSize:                       8 * mem.GB,
	}
	return b
}

// Build creates a pre-configure GPU similar to the AMD R9 Nano GPU.
func (b R9NanoGPUBuilder) Build(name string, id uint64) *GPU {
	b.createGPU(name, id)
	b.buildSAs()
	b.buildL2Caches()
	b.buildDRAMControllers()
	b.buildCP()
	b.buildGMMU()
	b.buildGMMUCache()
	b.buildL2TLB()

	b.connectCP()
	b.connectCPWithCUs()
	b.connectCPWithAddressTranslators()
	b.connectCPWithTLBs()
	b.connectCPWithCaches()
	b.connectL2AndDRAM()
	b.connectL1ToL2()
	b.connectL1TLBToL2TLB()
	b.connectL2TLBToGMMUCache()
	b.connectGMMUCachetoGMMU()

	b.populateExternalPorts()

	return b.gpu
}

func (b *R9NanoGPUBuilder) populateExternalPorts() {
	b.gpu.Domain.AddPort("CommandProcessor", b.cp.ToDriver)
	b.gpu.Domain.AddPort("RDMA", b.rdmaEngine.ToOutside)
	b.gpu.Domain.AddPort("PageMigrationController",
		b.pageMigrationController.GetPortByName("Remote"))

	// for i, l2TLB := range b.l2TLBs {
	// 	name := fmt.Sprintf("Translation[%02d]", i)
	// 	b.gpu.Domain.AddPort(name, l2TLB.GetPortByName("Bottom"))
	// }
	// for i, gmmu := range b.gmmu {
	// 	name := fmt.Sprintf("GMMU[%02d]", i)
	// 	b.gpu.Domain.AddPort(name, gmmu.GetPortByName("Bottom"))
	// }
	b.gpu.Domain.AddPort("GMMU", b.gmmu.GetPortByName("Bottom"))
}

func (b *R9NanoGPUBuilder) createGPU(name string, id uint64) {
	b.gpuName = name

	b.gpu = &GPU{}
	b.gpu.Domain = sim.NewDomain(b.gpuName)
	b.gpuID = id
}

func (b *R9NanoGPUBuilder) populateCUs(sa *shaderArray) {
	for _, cu := range sa.cus {
		b.cus = append(b.cus, cu)
		b.gpu.CUs = append(b.gpu.CUs, cu)

		if b.monitor != nil {
			b.monitor.RegisterComponent(cu)
		}

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(cu)
		}
	}
}

func (b *R9NanoGPUBuilder) populateROBs(sa *shaderArray) {
	for _, rob := range sa.l1vROBs {
		b.l1vReorderBuffers = append(b.l1vReorderBuffers, rob)

		if b.monitor != nil {
			b.monitor.RegisterComponent(rob)
		}

		// if b.visTracer != nil {
		// 	tracing.CollectTrace(rob, b.visTracer)
		// }

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(rob)
		}
	}
}

func (b *R9NanoGPUBuilder) populateTLBs(sa *shaderArray) {
	for _, tlb := range sa.l1vTLBs {
		b.l1vTLBs = append(b.l1vTLBs, tlb)
		b.gpu.L1VTLBs = append(b.gpu.L1VTLBs, tlb)

		if b.monitor != nil {
			b.monitor.RegisterComponent(tlb)
		}

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(tlb)
		}
	}
}

func (b *R9NanoGPUBuilder) populateL1Vs(sa *shaderArray) {
	for _, l1v := range sa.l1vCaches {
		b.l1vCaches = append(b.l1vCaches, l1v)
		b.gpu.L1VCaches = append(b.gpu.L1VCaches, l1v)

		if b.monitor != nil {
			b.monitor.RegisterComponent(l1v)
		}

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(l1v)
		}
	}
}

func (b *R9NanoGPUBuilder) populateL1VAddressTranslators(sa *shaderArray) {
	for _, at := range sa.l1vATs {
		b.l1vAddrTrans = append(b.l1vAddrTrans, at)

		if b.monitor != nil {
			b.monitor.RegisterComponent(at)
		}

		if b.perfAnalyzer != nil {
			b.perfAnalyzer.RegisterComponent(at)
		}
	}
}

func (b *R9NanoGPUBuilder) populateScalerMemoryHierarchy(sa *shaderArray) {
	b.l1sAddrTrans = append(b.l1sAddrTrans, sa.l1sAT)
	b.l1sReorderBuffers = append(b.l1sReorderBuffers, sa.l1sROB)
	b.l1sCaches = append(b.l1sCaches, sa.l1sCache)
	b.gpu.L1SCaches = append(b.gpu.L1SCaches, sa.l1sCache)
	b.l1sTLBs = append(b.l1sTLBs, sa.l1sTLB)
	b.gpu.L1STLBs = append(b.gpu.L1STLBs, sa.l1sTLB)

	if b.monitor != nil {
		b.monitor.RegisterComponent(sa.l1sAT)
		b.monitor.RegisterComponent(sa.l1sROB)
		b.monitor.RegisterComponent(sa.l1sCache)
		b.monitor.RegisterComponent(sa.l1sTLB)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(sa.l1sAT)
		b.perfAnalyzer.RegisterComponent(sa.l1sROB)
		b.perfAnalyzer.RegisterComponent(sa.l1sCache)
		b.perfAnalyzer.RegisterComponent(sa.l1sTLB)
	}

	// if b.visTracer != nil {
	// 	tracing.CollectTrace(sa.l1sAT, b.visTracer)
	// 	tracing.CollectTrace(sa.l1sROB, b.visTracer)
	// 	tracing.CollectTrace(sa.l1sCache, b.visTracer)
	// 	tracing.CollectTrace(sa.l1sTLB, b.visTracer)
	// }
}

func (b *R9NanoGPUBuilder) populateInstMemoryHierarchy(sa *shaderArray) {
	b.l1iAddrTrans = append(b.l1iAddrTrans, sa.l1iAT)
	b.l1iReorderBuffers = append(b.l1iReorderBuffers, sa.l1iROB)
	b.l1iCaches = append(b.l1iCaches, sa.l1iCache)
	b.gpu.L1ICaches = append(b.gpu.L1ICaches, sa.l1iCache)
	b.l1iTLBs = append(b.l1iTLBs, sa.l1iTLB)
	b.gpu.L1ITLBs = append(b.gpu.L1ITLBs, sa.l1iTLB)

	if b.monitor != nil {
		b.monitor.RegisterComponent(sa.l1iAT)
		b.monitor.RegisterComponent(sa.l1iROB)
		b.monitor.RegisterComponent(sa.l1iCache)
		b.monitor.RegisterComponent(sa.l1iTLB)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(sa.l1iAT)
		b.perfAnalyzer.RegisterComponent(sa.l1iROB)
		b.perfAnalyzer.RegisterComponent(sa.l1iCache)
		b.perfAnalyzer.RegisterComponent(sa.l1iTLB)
	}

	// if b.visTracer != nil {
	// 	tracing.CollectTrace(sa.l1iAT, b.visTracer)
	// 	tracing.CollectTrace(sa.l1iROB, b.visTracer)
	// 	tracing.CollectTrace(sa.l1iCache, b.visTracer)
	// 	tracing.CollectTrace(sa.l1iTLB, b.visTracer)
	// }
}

func (b *R9NanoGPUBuilder) numCU() int {
	return b.numCUPerShaderArray * b.numShaderArray
}
