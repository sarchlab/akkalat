package gpuArch

import (
	"github.com/sarchlab/akita/v3/analysis"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/mem/vm/mmu"
	"github.com/sarchlab/akita/v3/monitoring"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
)

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

// WithNumShaderArray sets the number of shader arrays in each GPU. Each shader
// array contains a certain number of CUs, a certain number of L1V caches, 1
// L1S cache, and 1 L1V cache.
func (b R9NanoGPUBuilder) WithNumShaderArray(n int) R9NanoGPUBuilder {
	b.numShaderArray = n
	return b
}

// WithNumCUPerShaderArray sets the number of CU and number of L1V caches in
// each Shader Array.
func (b R9NanoGPUBuilder) WithNumCUPerShaderArray(n int) R9NanoGPUBuilder {
	b.numCUPerShaderArray = n
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

// WithLog2CacheLineSize sets the cache line size with the power of 2.
func (b R9NanoGPUBuilder) WithLog2CacheLineSize(
	log2CacheLine uint64,
) R9NanoGPUBuilder {
	b.log2CacheLineSize = log2CacheLine
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

func (b R9NanoGPUBuilder) WithPerfAnalyzer(
	a *analysis.PerfAnalyzer,
) R9NanoGPUBuilder {
	b.perfAnalyzer = a
	return b
}

// WithL2CacheSize set the total L2 cache size. The size of the L2 cache is
// split between memory banks.
func (b R9NanoGPUBuilder) WithL2CacheSize(size uint64) R9NanoGPUBuilder {
	b.l2CacheSize = size
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

func (b R9NanoGPUBuilder) WithGMMUPageTable(
	pageTable vm.PageTable,
) R9NanoGPUBuilder {
	b.pageTable = pageTable
	return b
}
