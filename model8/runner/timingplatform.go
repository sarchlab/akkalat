package runner

import (
	"fmt"
	"log"
	"os"

	"github.com/tebeka/atexit"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/sim/bottleneckanalysis"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	memtraces "gitlab.com/akita/mem/v3/trace"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mgpusim/v3/driver"
	"gitlab.com/akita/noc/v3/networking/noctracing"
	"gitlab.com/akita/noc/v3/networking/pcie"
)

// WaferScaleGPUPlatformBuilder can build a platform that equips Wafer-Scale GPU.
type WaferScaleGPUPlatformBuilder struct {
	useParallelEngine     bool
	debugISA              bool
	traceVis              bool
	visTraceStartTime     sim.VTimeInSec
	visTraceEndTime       sim.VTimeInSec
	traceNoC              bool
	traceMem              bool
	numGPU                int
	useMagicMemoryCopy    bool
	log2PageSize          uint64
	engine                sim.Engine
	bufferAnalyzingDir    string
	bufferAnalyzingPeriod float64
	bufferAnalyzer        *bottleneckanalysis.BufferAnalyzer

	tileWidth  int
	tileHeight int

	monitor *monitoring.Monitor

	globalStorage *mem.Storage

	gpus []*GPU
}

// MakeWaferScaleGPUBuilder creates a EmuBuilder with default parameters.
func MakeWaferScaleGPUPlatformBuilder() WaferScaleGPUPlatformBuilder {
	b := WaferScaleGPUPlatformBuilder{
		numGPU:            4,
		log2PageSize:      12,
		visTraceStartTime: -1,
		visTraceEndTime:   -1,
	}
	return b
}

// WithParallelEngine lets the EmuBuilder to use parallel engine.
func (b WaferScaleGPUPlatformBuilder) WithParallelEngine() WaferScaleGPUPlatformBuilder {
	b.useParallelEngine = true
	return b
}

// WithISADebugging enables ISA debugging in the simulation.
func (b WaferScaleGPUPlatformBuilder) WithISADebugging() WaferScaleGPUPlatformBuilder {
	b.debugISA = true
	return b
}

// WithVisTracing lets the platform to record traces for general visualization.
func (b WaferScaleGPUPlatformBuilder) WithVisTracing() WaferScaleGPUPlatformBuilder {
	b.traceVis = true
	return b
}

// WithPartialVisTracing lets the platform to record traces for general
// visualization. The trace will only be collected from the start time to the
// end time.
func (b WaferScaleGPUPlatformBuilder) WithPartialVisTracing(
	start, end sim.VTimeInSec,
) WaferScaleGPUPlatformBuilder {
	b.traceVis = true
	b.visTraceStartTime = start
	b.visTraceEndTime = end
	return b
}

// WithNoCTracing lets the platform to record traces for NoC visualization.
func (b WaferScaleGPUPlatformBuilder) WithNoCTracing() WaferScaleGPUPlatformBuilder {
	b.traceNoC = true
	return b
}

// WithPartialNoCTracing lets the platform to record traces for NoC visualization
// The trace will only be collected from the start time to the end time.
func (b WaferScaleGPUPlatformBuilder) WithPartialNoCTracing(
	start, end sim.VTimeInSec,
) WaferScaleGPUPlatformBuilder {
	b.traceNoC = true
	b.visTraceStartTime = start
	b.visTraceEndTime = end
	return b
}

// WithMemTracing lets the platform to trace memory operations.
func (b WaferScaleGPUPlatformBuilder) WithMemTracing() WaferScaleGPUPlatformBuilder {
	b.traceMem = true
	return b
}

// WithNumGPU sets the number of GPUs to build.
func (b WaferScaleGPUPlatformBuilder) WithNumGPU(n int) WaferScaleGPUPlatformBuilder {
	b.numGPU = n
	return b
}

// WithLog2PageSize sets the page size as a power of 2.
func (b WaferScaleGPUPlatformBuilder) WithLog2PageSize(
	n uint64,
) WaferScaleGPUPlatformBuilder {
	b.log2PageSize = n
	return b
}

// WithMonitor sets the monitor that is used to monitor the simulation
func (b WaferScaleGPUPlatformBuilder) WithMonitor(
	m *monitoring.Monitor,
) WaferScaleGPUPlatformBuilder {
	b.monitor = m
	return b
}

// WithMagicMemoryCopy uses global storage as memory components
func (b WaferScaleGPUPlatformBuilder) WithMagicMemoryCopy() WaferScaleGPUPlatformBuilder {
	b.useMagicMemoryCopy = true
	return b
}

// WithTileWidth sets the number of tiles horizontally.
func (b WaferScaleGPUPlatformBuilder) WithTileWidth(w int) WaferScaleGPUPlatformBuilder {
	b.tileWidth = w
	return b
}

// WithTileHeight sets the number of tiles vertically.
func (b WaferScaleGPUPlatformBuilder) WithTileHeight(h int) WaferScaleGPUPlatformBuilder {
	b.tileHeight = h
	return b
}

// WithBufferAnalyzer sets the trace that dumps the buffer levers.
func (b WaferScaleGPUPlatformBuilder) WithBufferAnalyzer(
	traceDirName string,
	tracePeriod float64,
) WaferScaleGPUPlatformBuilder {
	b.bufferAnalyzingDir = traceDirName
	b.bufferAnalyzingPeriod = tracePeriod
	return b
}

// Build builds a platform with Wafer-Scale GPUs.
func (b WaferScaleGPUPlatformBuilder) Build() *Platform {
	b.engine = b.createEngine()
	if b.monitor != nil {
		b.monitor.RegisterEngine(b.engine)
	}

	b.setupBufferLevelTracing()

	b.globalStorage = mem.NewStorage(uint64(1+b.numGPU) * 4 * mem.GB)

	mmuComponent, pageTable := b.createMMU(b.engine)

	gpuDriverBuilder := driver.MakeBuilder()
	if b.useMagicMemoryCopy {
		gpuDriverBuilder = gpuDriverBuilder.WithMagicMemoryCopyMiddleware()
	}
	gpuDriver := gpuDriverBuilder.
		WithEngine(b.engine).
		WithPageTable(pageTable).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage).
		Build("Driver")
	// file, err := os.Create("driver_comm.csv")
	// if err != nil {
	// 	panic(err)
	// }
	// gpuDriver.GetPortByName("GPU").AcceptHook(
	// 	sim.NewPortMsgLogger(log.New(file, "", 0)))

	if b.monitor != nil {
		b.monitor.RegisterComponent(gpuDriver)
	}

	gpuBuilder := b.createGPUBuilder(b.engine, gpuDriver, mmuComponent)
	pcieConnector, rootComplexID :=
		b.createConnection(b.engine, gpuDriver, mmuComponent)

	mmuComponent.MigrationServiceProvider = gpuDriver.GetPortByName("MMU")

	rdmaAddressTable := b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createGPUs(
		rootComplexID, pcieConnector,
		gpuBuilder, gpuDriver,
		rdmaAddressTable, pmcAddressTable)

	pcieConnector.EstablishRoute()

	return &Platform{
		Engine: b.engine,
		Driver: gpuDriver,
		GPUs:   b.gpus,
	}
}

func (b *WaferScaleGPUPlatformBuilder) setupBufferLevelTracing() {
	if b.bufferAnalyzingDir != "" {
		b.bufferAnalyzer = bottleneckanalysis.MakeBufferAnalyzerBuilder().
			WithTimeTeller(b.engine).
			WithPeriod(b.bufferAnalyzingPeriod).
			WithDirectoryPath(b.bufferAnalyzingDir).
			Build()

		atexit.Register(b.bufferAnalyzer.Report)
	}
}

func (b *WaferScaleGPUPlatformBuilder) createGPUs(
	rootComplexID int,
	pcieConnector *pcie.Connector,
	gpuBuilder WaferScaleGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
) {
	lastSwitchID := rootComplexID
	for i := 1; i < b.numGPU+1; i++ {
		if i%2 == 1 {
			lastSwitchID = pcieConnector.AddSwitch(rootComplexID)
		}

		b.createGPU(i, gpuBuilder, gpuDriver,
			rdmaAddressTable, pmcAddressTable,
			pcieConnector, lastSwitchID)
	}
}

func (b WaferScaleGPUPlatformBuilder) createPMCPageTable() *mem.BankedLowModuleFinder {
	pmcAddressTable := new(mem.BankedLowModuleFinder)
	pmcAddressTable.BankSize = 4 * mem.GB
	pmcAddressTable.LowModules = append(pmcAddressTable.LowModules, nil)
	return pmcAddressTable
}

func (b WaferScaleGPUPlatformBuilder) createRDMAAddrTable() *mem.BankedLowModuleFinder {
	rdmaAddressTable := new(mem.BankedLowModuleFinder)
	rdmaAddressTable.BankSize = 4 * mem.GB
	rdmaAddressTable.LowModules = append(rdmaAddressTable.LowModules, nil)
	return rdmaAddressTable
}

func (b WaferScaleGPUPlatformBuilder) createConnection(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
) (*pcie.Connector, int) {
	//connection := sim.NewDirectConnection(engine)
	// connection := noc.NewFixedBandwidthConnection(32, engine, 1*sim.GHz)
	// connection.SrcBufferCapacity = 40960000
	pcieConnector := pcie.NewConnector().
		WithEngine(engine).
		WithVersion(3, 16).
		WithSwitchLatency(140)
	pcieConnector.CreateNetwork("PCIe")
	rootComplexID := pcieConnector.AddRootComplex(
		[]sim.Port{
			gpuDriver.GetPortByName("GPU"),
			gpuDriver.GetPortByName("MMU"),
			mmuComponent.GetPortByName("Migration"),
			mmuComponent.GetPortByName("Top"),
		})
	return pcieConnector, rootComplexID
}

func (b WaferScaleGPUPlatformBuilder) createEngine() sim.Engine {
	var engine sim.Engine

	if b.useParallelEngine {
		engine = sim.NewParallelEngine()
	} else {
		engine = sim.NewSerialEngine()
	}
	// engine.AcceptHook(sim.NewEventLogger(log.New(os.Stdout, "", 0)))

	return engine
}

func (b WaferScaleGPUPlatformBuilder) createMMU(
	engine sim.Engine,
) (*mmu.MMU, vm.PageTable) {
	pageTable := vm.NewPageTable(b.log2PageSize)
	mmuBuilder := mmu.MakeBuilder().
		WithEngine(engine).
		WithFreq(1 * sim.GHz).
		WithPageWalkingLatency(100).
		WithLog2PageSize(b.log2PageSize).
		WithPageTable(pageTable)

	mmuComponent := mmuBuilder.Build("MMU")

	if b.monitor != nil {
		b.monitor.RegisterComponent(mmuComponent)
	}

	return mmuComponent, pageTable
}

func (b *WaferScaleGPUPlatformBuilder) createGPUBuilder(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
) WaferScaleGPUBuilder {
	gpuBuilder := MakeWaferScaleGPUBuilder().
		WithEngine(engine).
		WithMMU(mmuComponent).
		WithNumMemoryBank(16).
		WithLog2MemoryBankInterleavingSize(7).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage).
		WithTileWidth(b.tileWidth).
		WithTileHeight(b.tileHeight)

	if b.monitor != nil {
		gpuBuilder = gpuBuilder.WithMonitor(b.monitor)
	}

	if b.bufferAnalyzer != nil {
		gpuBuilder = gpuBuilder.WithBufferAnalyzer(b.bufferAnalyzer)
	}

	gpuBuilder = b.setVisTracer(gpuDriver, gpuBuilder)
	gpuBuilder = b.setNoCTracer(gpuBuilder)
	gpuBuilder = b.setMemTracer(gpuBuilder)
	gpuBuilder = b.setISADebugger(gpuBuilder)

	return gpuBuilder
}

func (b *WaferScaleGPUPlatformBuilder) setISADebugger(
	gpuBuilder WaferScaleGPUBuilder,
) WaferScaleGPUBuilder {
	if !b.debugISA {
		return gpuBuilder
	}

	gpuBuilder = gpuBuilder.WithISADebugging()
	return gpuBuilder
}

func (b *WaferScaleGPUPlatformBuilder) setVisTracer(
	gpuDriver *driver.Driver,
	gpuBuilder WaferScaleGPUBuilder,
) WaferScaleGPUBuilder {
	if !b.traceVis {
		return gpuBuilder
	}

	mySQLTracer := tracing.NewMySQLTracerWithTimeRange(
		b.engine,
		b.visTraceStartTime,
		b.visTraceEndTime,
	)
	mySQLTracer.Init()
	tracing.CollectTrace(gpuDriver, mySQLTracer)

	// Use CSV tracer to dump all tracing data into a file, with similar format to
	// MySQL tracer. The mesh networking data will be saves meanwhile.

	// tracer := tracing.NewCSVTracer()
	// tracer.Init()

	gpuBuilder = gpuBuilder.WithVisTracer(mySQLTracer)
	return gpuBuilder
}

func (b *WaferScaleGPUPlatformBuilder) setNoCTracer(
	gpuBuilder WaferScaleGPUBuilder,
) WaferScaleGPUBuilder {
	if !b.traceNoC {
		return gpuBuilder
	}

	nocTracer := noctracing.NewMeshNetworkTracerWithTimeRange(
		b.engine,
		b.visTraceStartTime,
		b.visTraceEndTime,
		16,
		uint16(b.tileWidth),
		uint16(b.tileHeight),
		*nocTracingOutputDir,
	)
	nocTracer.Init()

	gpuBuilder = gpuBuilder.WithNoCTracer(nocTracer)
	return gpuBuilder
}

func (b *WaferScaleGPUPlatformBuilder) setMemTracer(
	gpuBuilder WaferScaleGPUBuilder,
) WaferScaleGPUBuilder {
	if !b.traceMem {
		return gpuBuilder
	}

	file, err := os.Create("mem.trace")
	if err != nil {
		panic(err)
	}
	logger := log.New(file, "", 0)
	memTracer := memtraces.NewTracer(logger, b.engine)
	gpuBuilder = gpuBuilder.WithMemTracer(memTracer)
	return gpuBuilder
}

func (b *WaferScaleGPUPlatformBuilder) createGPU(
	index int,
	gpuBuilder WaferScaleGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
	pcieConnector *pcie.Connector,
	pcieSwitchID int,
) *GPU {
	name := fmt.Sprintf("GPU%d", index)
	memAddrOffset := uint64(index) * 4 * mem.GB
	gpu := gpuBuilder.
		WithMemAddrOffset(memAddrOffset).
		Build(name, uint64(index))
	gpuDriver.RegisterGPU(
		gpu.Domain.GetPortByName("CommandProcessor"),
		driver.DeviceProperties{
			CUCount:  b.tileHeight * b.tileWidth,
			DRAMSize: 4 * mem.GB,
		},
	)
	gpu.CommandProcessor.Driver = gpuDriver.GetPortByName("GPU")

	b.configRDMAEngine(gpu, rdmaAddressTable)
	b.configPMC(gpu, gpuDriver, pmcAddressTable)

	pcieConnector.PlugInDevice(pcieSwitchID, gpu.Domain.Ports())

	b.gpus = append(b.gpus, gpu)

	return gpu
}

func (b *WaferScaleGPUPlatformBuilder) configRDMAEngine(
	gpu *GPU,
	addrTable *mem.BankedLowModuleFinder,
) {
	gpu.RDMAEngine.RemoteRDMAAddressTable = addrTable
	addrTable.LowModules = append(
		addrTable.LowModules,
		gpu.RDMAEngine.ToOutside)
}

func (b *WaferScaleGPUPlatformBuilder) configPMC(
	gpu *GPU,
	gpuDriver *driver.Driver,
	addrTable *mem.BankedLowModuleFinder,
) {
	gpu.PMC.RemotePMCAddressTable = addrTable
	addrTable.LowModules = append(
		addrTable.LowModules,
		gpu.PMC.GetPortByName("Remote"))
	gpuDriver.RemotePMCPorts = append(
		gpuDriver.RemotePMCPorts, gpu.PMC.GetPortByName("Remote"))
}
