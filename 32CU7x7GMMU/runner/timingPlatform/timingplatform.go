package timingPlatform

import (
	"fmt"
	"log"
	"os"

	"github.com/sarchlab/akita/v4/datarecording"
	memtraces "github.com/sarchlab/akita/v4/mem/trace"
	"github.com/sarchlab/akkalat/32CU7x7GMMU/runner/gpuArch"

	"github.com/sarchlab/akita/v4/analysis"
	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/mem/vm/mmu"
	"github.com/sarchlab/akita/v4/monitoring"
	mesh "github.com/sarchlab/akita/v4/noc/networking/mesh"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/driver"
)

// R9NanoPlatformBuilder can build a platform that equips R9Nano GPU.
type R9NanoPlatformBuilder struct {
	useParallelEngine     bool
	debugISA              bool
	traceVis              bool
	visTraceStartTime     sim.VTimeInSec
	visTraceEndTime       sim.VTimeInSec
	traceMem              bool
	tileWidth, tileHeight int
	numSAPerGPU           int
	numCUPerSA            int
	useMagicMemoryCopy    bool
	log2PageSize          uint64
	bandwidth             int
	switchLatency         int
	maxNumHops            int

	engine    sim.Engine
	visTracer tracing.Tracer
	monitor   *monitoring.Monitor

	globalStorage *mem.Storage

	perfAnalysisFileName string
	perfAnalyzingPeriod  float64
	perfAnalyzer         *analysis.PerfAnalyzer

	visTracerDBFileName string
	visTracerDB         string

	gpus []*gpuArch.GPU
}

// MakeR9NanoBuilder creates a EmuBuilder with default parameters.
func MakeR9NanoBuilder() R9NanoPlatformBuilder {
	b := R9NanoPlatformBuilder{
		tileWidth:         7,
		tileHeight:        7,
		log2PageSize:      12,
		visTraceStartTime: -1,
		visTraceEndTime:   -1,
		switchLatency:     20,
		numSAPerGPU:       8,
		numCUPerSA:        4,
		maxNumHops:        -1,
	}
	return b
}

// Build builds a platform with R9Nano GPUs.
func (b R9NanoPlatformBuilder) Build(numMemoryBank int) *gpuArch.Platform {
	b.engine = b.createEngine()
	if b.monitor != nil {
		b.monitor.RegisterEngine(b.engine)
	}

	b.setupVisTracing()
	b.setupPerfermanceTracing()

	numGPU := b.tileWidth*b.tileHeight - 1
	b.globalStorage = mem.NewStorage(uint64(1+numGPU) * 8 * mem.GB)

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
		WithD2HCycles(8500).
		WithH2DCycles(14500).
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

	connector := b.createConnection(b.engine, gpuDriver, mmuComponent)
	gpuBuilder := b.createGPUBuilder(b.engine, gpuDriver, mmuComponent, numMemoryBank, pageTable)
	mmuComponent.MigrationServiceProvider = gpuDriver.GetPortByName("MMU").AsRemote()

	rdmaAddressTable := b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createGPUs(
		connector,
		gpuBuilder, gpuDriver,
		rdmaAddressTable, pmcAddressTable)

	connector.EstablishNetwork()

	for _, gpu := range b.gpus {
		gpu.MMUEngine = mmuComponent
	}

	return &gpuArch.Platform{
		Engine: b.engine,
		Driver: gpuDriver,
		GPUs:   b.gpus,
	}
}

func (b *R9NanoPlatformBuilder) setupVisTracing() {
	if !b.traceVis {
		return
	}

	dataRecorder := datarecording.NewDataRecorder("simulation.sqlite3")
	visTracer := tracing.NewDBTracer(b.engine, dataRecorder)
	visTracer.SetTimeRange(b.visTraceStartTime, b.visTraceEndTime)

	b.visTracer = visTracer
}

func (b *R9NanoPlatformBuilder) createGPUs(
	connector *mesh.Connector,
	gpuBuilder gpuArch.R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedAddressPortMapper,
	pmcAddressTable *mem.BankedAddressPortMapper,
) {
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}

			b.createGPU(x, y, gpuBuilder, gpuDriver, rdmaAddressTable, pmcAddressTable, connector)
		}
	}
}

func (b R9NanoPlatformBuilder) createPMCPageTable() *mem.BankedAddressPortMapper {
	pmcAddressTable := new(mem.BankedAddressPortMapper)
	pmcAddressTable.BankSize = 8 * mem.GB
	pmcAddressTable.LowModules = append(pmcAddressTable.LowModules, "")
	return pmcAddressTable
}

func (b R9NanoPlatformBuilder) createRDMAAddrTable() *mem.BankedAddressPortMapper {
	rdmaAddressTable := new(mem.BankedAddressPortMapper)
	rdmaAddressTable.BankSize = 8 * mem.GB
	rdmaAddressTable.LowModules = append(rdmaAddressTable.LowModules, "")
	return rdmaAddressTable
}

func (b R9NanoPlatformBuilder) createConnection(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.Comp,
) *mesh.Connector {
	connector := mesh.NewConnector().
		WithEngine(engine).
		WithFreq(1 * sim.GHz).
		WithFlitSize(16).
		WithBandwidth(float64(b.bandwidth)).
		WithSwitchLatency(b.switchLatency)

	if b.traceVis {
		connector = connector.WithVisTracer(b.visTracer)
	}

	connector.CreateNetwork("Mesh")
	connector.AddTile([3]int{b.tileWidth / 2, b.tileHeight / 2, 0}, []sim.Port{
		gpuDriver.GetPortByName("GPU"),
		gpuDriver.GetPortByName("MMU"),
		mmuComponent.GetPortByName("Migration"),
		mmuComponent.GetPortByName("Top"),
	})

	return connector
}

func (b R9NanoPlatformBuilder) createEngine() sim.Engine {
	var engine sim.Engine

	if b.useParallelEngine {
		engine = sim.NewParallelEngine()
	} else {
		engine = sim.NewSerialEngine()
	}
	// engine.AcceptHook(sim.NewEventLogger(log.New(os.Stdout, "", 0)))

	return engine
}

func (b R9NanoPlatformBuilder) createMMU(
	engine sim.Engine,
) (*mmu.Comp, vm.PageTable) {
	pageTable := vm.NewPageTable(b.log2PageSize)
	mmuBuilder := mmu.MakeBuilder().
		WithEngine(engine).
		WithFreq(1 * sim.GHz).
		WithPageWalkingLatency(100).
		WithLog2PageSize(b.log2PageSize).
		WithMaxNumReqInFlight(8196000).
		WithPageTable(pageTable)

	mmuComponent := mmuBuilder.Build("MMU")

	if b.monitor != nil {
		b.monitor.RegisterComponent(mmuComponent)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(mmuComponent)
	}

	if b.visTracer != nil {
		tracing.CollectTrace(mmuComponent, b.visTracer)
	}

	return mmuComponent, pageTable
}

func (b *R9NanoPlatformBuilder) createGPUBuilder(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.Comp,
	numMemoryBank int,
	pageTable vm.PageTable,
) gpuArch.R9NanoGPUBuilder {
	gpuBuilder := gpuArch.MakeR9NanoGPUBuilder().
		WithEngine(engine).
		WithMMU(mmuComponent).
		WithNumCUPerShaderArray(b.numCUPerSA).
		WithNumShaderArray(b.numSAPerGPU).
		WithNumMemoryBank(numMemoryBank).
		WithL2CacheSize(4 * mem.MB).
		WithLog2MemoryBankInterleavingSize(7).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage).
		WithPerfAnalyzer(b.perfAnalyzer).
		WithGMMUPageTable(pageTable)

	if b.monitor != nil {
		gpuBuilder = gpuBuilder.WithMonitor(b.monitor)
	}

	gpuBuilder = b.setVisTracer(gpuDriver, gpuBuilder)
	gpuBuilder = b.setMemTracer(gpuBuilder)
	gpuBuilder = b.setISADebugger(gpuBuilder)

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) setISADebugger(
	gpuBuilder gpuArch.R9NanoGPUBuilder,
) gpuArch.R9NanoGPUBuilder {
	if !b.debugISA {
		return gpuBuilder
	}

	gpuBuilder = gpuBuilder.WithISADebugging()
	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) setMemTracer(
	gpuBuilder gpuArch.R9NanoGPUBuilder,
) gpuArch.R9NanoGPUBuilder {
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

func (b *R9NanoPlatformBuilder) setVisTracer(
	gpuDriver *driver.Driver,
	gpuBuilder gpuArch.R9NanoGPUBuilder,
) gpuArch.R9NanoGPUBuilder {
	if b.traceVis {
		gpuBuilder = gpuBuilder.WithVisTracer(b.visTracer)
	}

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) createGPU(
	x, y int,
	gpuBuilder gpuArch.R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedAddressPortMapper,
	pmcAddressTable *mem.BankedAddressPortMapper,
	connector *mesh.Connector,
) *gpuArch.GPU {
	index := uint64(len(b.gpus)) + 1
	gpuid := x + y*b.tileWidth
	name := fmt.Sprintf("GPU[%d]", gpuid)
	memAddrOffset := index * 8 * mem.GB
	fmt.Printf("GPU[%d], index %d, memAddrOffset %X\n", gpuid, index, memAddrOffset)
	gpu := gpuBuilder.
		WithMemAddrOffset(memAddrOffset).
		Build(name, uint64(index))
	gpuDriver.RegisterGPU(gpu.Domain.GetPortByName("CommandProcessor"),
		driver.DeviceProperties{
			CUCount:  32,
			DRAMSize: 8 * mem.GB,
		})
	gpu.CommandProcessor.Driver = gpuDriver.GetPortByName("GPU")

	b.configRDMAEngine(gpu, rdmaAddressTable)
	b.configPMC(gpu, gpuDriver, pmcAddressTable)

	connector.AddTile([3]int{x, y, 0}, gpu.Domain.Ports())

	b.gpus = append(b.gpus, gpu)

	return gpu
}

func (b *R9NanoPlatformBuilder) configRDMAEngine(
	gpu *gpuArch.GPU,
	addrTable *mem.BankedAddressPortMapper,
) {
	gpu.RDMAEngine.RemoteRDMAAddressTable = addrTable

	addrTable.LowModules = append(
		addrTable.LowModules,
		gpu.RDMAEngine.ToOutside.AsRemote())
}

func (b *R9NanoPlatformBuilder) configPMC(
	gpu *gpuArch.GPU,
	gpuDriver *driver.Driver,
	addrTable *mem.BankedAddressPortMapper,
) {
	gpu.PMC.RemotePMCAddressTable = addrTable
	addrTable.LowModules = append(
		addrTable.LowModules,
		gpu.PMC.GetPortByName("Remote").AsRemote())
	gpuDriver.RemotePMCPorts = append(
		gpuDriver.RemotePMCPorts, gpu.PMC.GetPortByName("Remote"))
}

func (b *R9NanoPlatformBuilder) setupPerfermanceTracing() {

	if b.perfAnalysisFileName != "" {
		b.perfAnalyzer = analysis.MakePerfAnalyzerBuilder().
			WithPeriod(sim.VTimeInSec(b.perfAnalyzingPeriod)).
			WithDBFilename(b.perfAnalysisFileName).
			WithEngine(b.engine).
			Build()
	}
}
