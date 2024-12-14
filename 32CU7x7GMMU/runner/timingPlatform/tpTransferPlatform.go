package timingPlatform

import (
	"github.com/sarchlab/akita/v3/monitoring"
	"github.com/sarchlab/akita/v3/sim"
)

// WithParallelEngine lets the EmuBuilder to use parallel engine.
func (b R9NanoPlatformBuilder) WithParallelEngine() R9NanoPlatformBuilder {
	b.useParallelEngine = true
	return b
}

// WithISADebugging enables ISA debugging in the simulation.
func (b R9NanoPlatformBuilder) WithISADebugging() R9NanoPlatformBuilder {
	b.debugISA = true
	return b
}

// WithVisTracing lets the platform to record traces for visualization purposes.
func (b R9NanoPlatformBuilder) WithVisTracing() R9NanoPlatformBuilder {
	b.traceVis = true
	return b
}

// WithPartialVisTracing lets the platform to record traces for visualization
// purposes. The trace will only be collected from the start time to the end
// time.
func (b R9NanoPlatformBuilder) WithPartialVisTracing(
	start, end sim.VTimeInSec,
) R9NanoPlatformBuilder {
	b.traceVis = true
	b.visTraceStartTime = start
	b.visTraceEndTime = end

	return b
}

// WithMemTracing lets the platform to trace memory operations.
func (b R9NanoPlatformBuilder) WithMemTracing() R9NanoPlatformBuilder {
	b.traceMem = true
	return b
}

// WithLog2PageSize sets the page size as a power of 2.
func (b R9NanoPlatformBuilder) WithLog2PageSize(
	n uint64,
) R9NanoPlatformBuilder {
	b.log2PageSize = n
	return b
}

// WithMonitor sets the monitor that is used to monitor the simulation
func (b R9NanoPlatformBuilder) WithMonitor(
	m *monitoring.Monitor,
) R9NanoPlatformBuilder {
	b.monitor = m
	return b
}

func (b R9NanoPlatformBuilder) WithPerfAnalyzer(
	Name string,
	Period float64,
) R9NanoPlatformBuilder {
	b.perfAnalysisFileName = Name
	b.perfAnalyzingPeriod = Period
	return b
}

// WithMagicMemoryCopy uses global storage as memory components
func (b R9NanoPlatformBuilder) WithMagicMemoryCopy() R9NanoPlatformBuilder {
	b.useMagicMemoryCopy = true
	return b
}

// WithBandwidth sets the bandwidth between adjacent GPUs in the unit of 16GB/s.
func (b R9NanoPlatformBuilder) WithBandwidth(
	bandwidth int,
) R9NanoPlatformBuilder {
	b.bandwidth = bandwidth
	return b
}

// WithSwitchLatency sets the switch latency.
func (b R9NanoPlatformBuilder) WithSwitchLatency(
	latency int,
) R9NanoPlatformBuilder {
	b.switchLatency = latency
	return b
}

// WithMaxNumHops sets the maximum number of hops that a flit can travel in the
// mesh network.
func (b R9NanoPlatformBuilder) WithMaxNumHops(
	n int,
) R9NanoPlatformBuilder {
	b.maxNumHops = n
	return b
}

func (b R9NanoPlatformBuilder) WithVisTracerDBFileName(
	fileName string,
) R9NanoPlatformBuilder {
	b.visTracerDBFileName = fileName
	return b
}

func (b R9NanoPlatformBuilder) WithVisTracerDB(
	db string,
) R9NanoPlatformBuilder {
	b.visTracerDB = db
	return b
}
