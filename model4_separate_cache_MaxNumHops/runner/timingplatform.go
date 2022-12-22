package runner

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"

	memtraces "gitlab.com/akita/mem/v3/trace"

	// "/Users/daoxuanxu/GPU_simulator/src/mem/mem/lowmodulefinder.go"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mgpusim/v3/driver"
	"gitlab.com/akita/noc/v3/networking/mesh"
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
	switchLatency         int

	engine    sim.Engine
	visTracer tracing.Tracer
	monitor   *monitoring.Monitor

	globalStorage *mem.Storage

	gpus []*GPU
}

// MakeR9NanoBuilder creates a EmuBuilder with default parameters.
func MakeR9NanoBuilder() R9NanoPlatformBuilder {
	b := R9NanoPlatformBuilder{
		tileWidth:         5,
		tileHeight:        5,
		log2PageSize:      12,
		visTraceStartTime: -1,
		visTraceEndTime:   -1,
		switchLatency:     10,
		numSAPerGPU:       8,
		numCUPerSA:        4,
	}
	return b
}

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

// WithMagicMemoryCopy uses global storage as memory components
func (b R9NanoPlatformBuilder) WithMagicMemoryCopy() R9NanoPlatformBuilder {
	b.useMagicMemoryCopy = true
	return b
}

// WithSwitchLatency sets the switch latency.
func (b R9NanoPlatformBuilder) WithSwitchLatency(
	latency int,
) R9NanoPlatformBuilder {
	b.switchLatency = latency
	return b
}

// Build builds a platform with R9Nano GPUs.
func (b R9NanoPlatformBuilder) Build(numOfHops int) *Platform {
	b.engine = b.createEngine()
	if b.monitor != nil {
		b.monitor.RegisterEngine(b.engine)
	}

	b.createVisTracer()

	numGPU := b.tileWidth*b.tileHeight - 1
	b.globalStorage = mem.NewStorage(uint64(1+numGPU) * 4 * mem.GB)

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

	connector := b.createConnection(b.engine, gpuDriver, mmuComponent)

	gpuBuilder := b.createGPUBuilder(b.engine, gpuDriver, mmuComponent)

	mmuComponent.MigrationServiceProvider = gpuDriver.GetPortByName("MMU")

	rdmaAddressTable := b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createGPUs(
		connector,
		gpuBuilder, gpuDriver,
		rdmaAddressTable, pmcAddressTable, numOfHops)

	connector.EstablishNetwork()

	return &Platform{
		Engine: b.engine,
		Driver: gpuDriver,
		GPUs:   b.gpus,
	}
}

func (b *R9NanoPlatformBuilder) createVisTracer() {
	if !b.traceVis {
		return
	}

	tracer := tracing.NewMySQLTracerWithTimeRange(
		b.engine,
		b.visTraceStartTime,
		b.visTraceEndTime)
	tracer.Init()
	b.visTracer = tracer
}

func (b *R9NanoPlatformBuilder) createGPUs(
	connector *mesh.Connector,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.AddressBoundary,
	pmcAddressTable *mem.BankedLowModuleFinder,
	numOfHops int,
) {
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}
			b.createGPU(x, y, gpuBuilder, gpuDriver, rdmaAddressTable, pmcAddressTable, connector)
		}
	}
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}
			gpuID := b.convertCoodrinateToGPUID(x, y)
			b.gpus[gpuID].RDMAEngine.RemoteRDMAAddressTable = b.setRDMAAddrTable(x, y, 9)
		}
	}
}

func (b R9NanoPlatformBuilder) createPMCPageTable() *mem.BankedLowModuleFinder {
	pmcAddressTable := new(mem.BankedLowModuleFinder)
	pmcAddressTable.BankSize = 4 * mem.GB
	pmcAddressTable.LowModules = append(pmcAddressTable.LowModules, nil)
	return pmcAddressTable
}

func (b R9NanoPlatformBuilder) createRDMAAddrTable() *mem.AddressBoundary {

	rdmaAddressTable := new(mem.AddressBoundary)
	// rdmaAddressTable.BankSize = 4 * mem.GB
	rdmaAddressTable.LowAddr = append(rdmaAddressTable.LowAddr, uint64(0))
	rdmaAddressTable.HighAddr = append(rdmaAddressTable.HighAddr, uint64(4*mem.GB))
	rdmaAddressTable.Port = append(rdmaAddressTable.Port, nil)
	return rdmaAddressTable
}

func (b R9NanoPlatformBuilder) createConnection(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
) *mesh.Connector {
	connector := mesh.NewConnector().
		WithEngine(engine).
		WithFreq(1 * sim.GHz).
		WithFlitSize(16).
		WithBandwidth(1).
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

func (b *R9NanoPlatformBuilder) createGPUBuilder(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
) R9NanoGPUBuilder {
	gpuBuilder := MakeR9NanoGPUBuilder().
		WithEngine(engine).
		WithMMU(mmuComponent).
		WithNumCUPerShaderArray(b.numCUPerSA).
		WithNumShaderArray(b.numSAPerGPU).
		WithNumMemoryBank(8).
		WithL2CacheSize(2 * mem.MB).
		WithLog2MemoryBankInterleavingSize(7).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage)

	if b.monitor != nil {
		gpuBuilder = gpuBuilder.WithMonitor(b.monitor)
	}

	gpuBuilder = b.setVisTracer(gpuDriver, gpuBuilder)
	gpuBuilder = b.setMemTracer(gpuBuilder)
	gpuBuilder = b.setISADebugger(gpuBuilder)

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) setISADebugger(
	gpuBuilder R9NanoGPUBuilder,
) R9NanoGPUBuilder {
	if !b.debugISA {
		return gpuBuilder
	}

	gpuBuilder = gpuBuilder.WithISADebugging()
	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) setMemTracer(
	gpuBuilder R9NanoGPUBuilder,
) R9NanoGPUBuilder {
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
	gpuBuilder R9NanoGPUBuilder,
) R9NanoGPUBuilder {
	if b.traceVis {
		gpuBuilder = gpuBuilder.WithVisTracer(b.visTracer)
	}

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) createGPU(
	x, y int,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.AddressBoundary,
	pmcAddressTable *mem.BankedLowModuleFinder,
	connector *mesh.Connector,
) *GPU {
	index := uint64(len(b.gpus)) + 1
	name := fmt.Sprintf("GPU_%d_%d", x, y)
	memAddrOffset := index * 4 * mem.GB
	gpu := gpuBuilder.
		WithMemAddrOffset(memAddrOffset).
		Build(name, uint64(index))
	gpuDriver.RegisterGPU(gpu.Domain.GetPortByName("CommandProcessor"),
		driver.DeviceProperties{
			CUCount:  32,
			DRAMSize: 4 * mem.GB,
		})
	gpu.CommandProcessor.Driver = gpuDriver.GetPortByName("GPU")

	// b.configRDMAEngine(gpu, rdmaAddressTable)
	b.configPMC(gpu, gpuDriver, pmcAddressTable)

	connector.AddTile([3]int{x, y, 0}, gpu.Domain.Ports())

	b.gpus = append(b.gpus, gpu)

	return gpu
}

// func (b *R9NanoPlatformBuilder) configRDMAEngine(
// 	gpu *GPU,
// 	addrTable *mem.AddressBoundary,
// ) {
// 	gpu.RDMAEngine.RemoteRDMAAddressTable = addrTable
// 	addrTable.LowModules = append(
// 		addrTable.LowModules,
// 		gpu.RDMAEngine.ToOutside)
// }

func (b *R9NanoPlatformBuilder) configPMC(
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

//----------------------------------------MaxNumHops--------------------------------//
func (b *R9NanoPlatformBuilder) convertGPUIDToCoodrinate(gpuID int) (int, int) {
	var x, y int
	cpuLocation := ((b.tileWidth + 1) + (b.tileHeight + 1) - 1) / 2
	if gpuID < cpuLocation {
		y = (gpuID - 1) / (b.tileWidth + 1)
		x = (gpuID - 1) % (b.tileWidth + 1)
	} else {
		y = gpuID / (b.tileWidth + 1)
		x = gpuID % (b.tileWidth + 1)
	}
	return x, y
}

func (b *R9NanoPlatformBuilder) convertCoodrinateToGPUID(x int, y int) int {
	var gpuID int
	if y < b.tileHeight/2 {
		gpuID = y*(b.tileWidth+1) + x + 1
	}
	if y == b.tileHeight/2 && x < b.tileWidth/2 {
		gpuID = y*(b.tileWidth+1) + x + 1
	}
	if y == b.tileHeight/2 && x > b.tileWidth/2 {
		gpuID = y*(b.tileWidth+1) + x
	}
	if y > b.tileHeight/2 {
		gpuID = y*(b.tileWidth+1) + x
	}
	return gpuID
}

type gpuCoodrinate struct {
	gpuID int
	x     int
	y     int
}

func NewGPUCoodrinate(gpuID int, x int, y int) *gpuCoodrinate {
	return &gpuCoodrinate{
		gpuID: gpuID,
		x:     x,
		y:     y,
	}
}
func (b *R9NanoPlatformBuilder) gpuMatrix() []gpuCoodrinate {
	var gpuID int = 1
	gpuCoodrinateList := []gpuCoodrinate{}
	newGPUCoodrinate := NewGPUCoodrinate
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}
			gpuCoodrinateList = append(gpuCoodrinateList, *newGPUCoodrinate(gpuID, x, y))
			gpuID = gpuID + 1
		}
	}
	return gpuCoodrinateList
}

func (b *R9NanoPlatformBuilder) gpuMatrixPartation(gpuLocationX int, gpuLocationY int) ([]gpuCoodrinate, []gpuCoodrinate, []gpuCoodrinate, []gpuCoodrinate) {
	var gpuID int = 1
	gpuCoodrinateListUp := []gpuCoodrinate{}
	gpuCoodrinateListDown := []gpuCoodrinate{}
	gpuCoodrinateListLeft := []gpuCoodrinate{}
	gpuCoodrinateListRight := []gpuCoodrinate{}
	newGPUCoodrinate := NewGPUCoodrinate
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}
			if y < gpuLocationY {
				gpuCoodrinateListUp = append(gpuCoodrinateListUp, *newGPUCoodrinate(gpuID, x, y))
			}
			if y > gpuLocationY {
				gpuCoodrinateListDown = append(gpuCoodrinateListDown, *newGPUCoodrinate(gpuID, x, y))
			}
			if x < gpuLocationX {
				gpuCoodrinateListLeft = append(gpuCoodrinateListLeft, *newGPUCoodrinate(gpuID, x, y))
			}
			if x > gpuLocationX {
				gpuCoodrinateListRight = append(gpuCoodrinateListRight, *newGPUCoodrinate(gpuID, x, y))
			}
			gpuID = gpuID + 1
		}
	}
	return gpuCoodrinateListUp, gpuCoodrinateListDown, gpuCoodrinateListLeft, gpuCoodrinateListRight
}

func (b *R9NanoPlatformBuilder) getCandidates(gpuLocationX int, gpuLocationY int, maxNumHops int, gpuCoorinateList []gpuCoodrinate) []int {
	candidates := make([]int, 0)
	for _, coodrinate := range gpuCoorinateList {
		gpuID := coodrinate.gpuID
		x := coodrinate.x
		y := coodrinate.y
		hops := int(math.Abs(float64(x-gpuLocationX)) + math.Abs(float64(y-gpuLocationY)))
		if hops == maxNumHops {
			candidates = append(candidates, gpuID)
		}
	}
	return candidates
}
func (b *R9NanoPlatformBuilder) sellectPortGPU(gpuLocationX int, gpuLocationY int, maxNumHops int) (int, int, int, int) {
	var upPort, downPort, leftPort, rightPort = 0, 0, 0, 0

	gpuCoodrinateListUp, gpuCoodrinateListDown, gpuCoodrinateListLeft, gpuCoodrinateListRight := b.gpuMatrixPartation(gpuLocationX, gpuLocationY)
	if gpuLocationX == 0 && gpuLocationY == 0 {
		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
		downPort = downPortCandidate[0]
		for _, rightPorts := range rightPortCandidate {
			if rightPorts == downPort {
				continue
			} else {
				rightPort = rightPorts
				break
			}
		}
	}

	if gpuLocationX == b.tileWidth && gpuLocationY == 0 {
		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
		downPort = downPortCandidate[0]
		for _, leftPorts := range leftPortCandidate {
			if leftPorts == downPort {
				continue
			} else {
				leftPort = leftPorts
				break
			}
		}
	}

	if gpuLocationX == 0 && gpuLocationY == b.tileHeight {
		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
		upPort = upPortCandidate[0]
		for _, rightPorts := range rightPortCandidate {
			if rightPorts == upPort {
				continue
			} else {
				rightPort = rightPorts
				break
			}
		}
	}

	if gpuLocationX == b.tileWidth && gpuLocationY == b.tileHeight {
		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
		upPort = upPortCandidate[0]
		for _, leftPorts := range leftPortCandidate {
			if leftPorts == upPort {
				continue
			} else {
				leftPort = leftPorts
				break
			}
		}
	}

	if gpuLocationX != 0 && gpuLocationX != b.tileWidth && gpuLocationY == 0 {
		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
		leftPort = leftPortCandidate[0]
		for _, rightPorts := range rightPortCandidate {
			if rightPorts == leftPort {
				continue
			} else {
				rightPort = rightPorts
				break
			}
		}

		for _, downPorts := range downPortCandidate {
			if downPorts == leftPort || downPorts == rightPort {
				continue
			} else {
				downPort = downPorts
				break
			}
		}
	}

	if gpuLocationX != 0 && gpuLocationX != b.tileWidth && gpuLocationY == b.tileHeight {
		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
		leftPort = leftPortCandidate[0]
		for _, rightPorts := range rightPortCandidate {
			if rightPorts == leftPort {
				continue
			} else {
				rightPort = rightPorts
				break
			}
		}

		for _, upPorts := range upPortCandidate {
			if upPorts == leftPort || upPorts == rightPort {
				continue
			} else {
				upPort = upPorts
				break
			}
		}
	}

	if gpuLocationY != 0 && gpuLocationY != b.tileHeight && gpuLocationX == 0 {
		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
		upPort = upPortCandidate[0]
		for _, downPorts := range downPortCandidate {
			if downPorts == leftPort {
				continue
			} else {
				downPort = downPorts
				break
			}
		}

		for _, rightPorts := range rightPortCandidate {
			if rightPorts == upPort || rightPorts == downPort {
				continue
			} else {
				rightPort = rightPorts
				break
			}
		}
	}

	if gpuLocationY != 0 && gpuLocationY != b.tileHeight && gpuLocationX == b.tileWidth {
		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
		upPort = upPortCandidate[0]
		for _, downPorts := range downPortCandidate {
			if downPorts == leftPort {
				continue
			} else {
				downPort = downPorts
				break
			}
		}

		for _, leftPorts := range leftPortCandidate {
			if leftPorts == upPort || leftPorts == downPort {
				continue
			} else {
				leftPort = leftPorts
				break
			}
		}
	}

	if gpuLocationY != 0 && gpuLocationY != b.tileHeight && gpuLocationX != b.tileWidth && gpuLocationX != 0 {
		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
		upPort = upPortCandidate[0]
		for _, downPorts := range downPortCandidate {
			if downPorts == leftPort {
				continue
			} else {
				downPort = downPorts
				break
			}
		}

		for _, leftPorts := range leftPortCandidate {
			if leftPorts == upPort || leftPorts == downPort {
				continue
			} else {
				leftPort = leftPorts
				break
			}
		}

		for _, rightPorts := range rightPortCandidate {
			if rightPorts == upPort || rightPorts == downPort || rightPorts == leftPort {
				continue
			} else {
				rightPort = rightPorts
				break
			}
		}
	}

	return upPort, downPort, leftPort, rightPort
}

func (b *R9NanoPlatformBuilder) isContain(gpuID int, gpuCoodrinateList []gpuCoodrinate) bool {
	for _, gpu := range gpuCoodrinateList {
		if gpu.gpuID == gpuID {
			return true
		}
	}
	return false
}

func (b *R9NanoPlatformBuilder) setConnerRDMATable(
	gpuCoodrinateSide1 []gpuCoodrinate,
	gpuCoodrinateSide2 []gpuCoodrinate,
	gpuPort1 int, gpuPort2 int, maxNumHops int, gpuLocationX int, gpuLocationY int) mem.AddressBoundary {
	rdmaAddrTable := mem.AddressBoundary{}
	// newrdmaAddrTable := mem.NewAddressBoundary
	gpuMatrix := b.gpuMatrix()
	for _, gpu := range gpuMatrix {
		assignedgpuID := gpu.gpuID
		x, y := b.convertGPUIDToCoodrinate(assignedgpuID)
		hops := int(math.Abs(float64(x-gpuLocationX) + math.Abs(float64(y-gpuLocationY))))
		if hops <= maxNumHops {
			rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
			rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
			rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[assignedgpuID].RDMAEngine.ToOutside)
		} else {
			if b.isContain(assignedgpuID, gpuCoodrinateSide1) && !b.isContain(assignedgpuID, gpuCoodrinateSide2) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
				continue
			}

			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) && b.isContain(assignedgpuID, gpuCoodrinateSide2) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
				continue
			}

			if b.isContain(assignedgpuID, gpuCoodrinateSide1) && b.isContain(assignedgpuID, gpuCoodrinateSide2) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
					continue
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
					continue
				}
			}
		}
	}
	return rdmaAddrTable
}

func (b *R9NanoPlatformBuilder) setSideRDMAAddrTable(
	gpuCoodrinateSide1 []gpuCoodrinate,
	gpuCoodrinateSide2 []gpuCoodrinate,
	gpuCoodrinateSide3 []gpuCoodrinate,
	gpuPort1 int,
	gpuPort2 int,
	gpuPort3 int,
	maxNumHops int, gpuLocationX int, gpuLocationY int,
) mem.AddressBoundary {
	rdmaAddrTable := mem.AddressBoundary{}
	gpuMatrix := b.gpuMatrix()
	for _, gpu := range gpuMatrix {
		assignedgpuID := gpu.gpuID
		x, y := b.convertGPUIDToCoodrinate(assignedgpuID)
		hops := int(math.Abs(float64(x-gpuLocationX) + math.Abs(float64(y-gpuLocationY))))
		if hops <= maxNumHops {
			rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
			rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
			rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[assignedgpuID].RDMAEngine.ToOutside)
		} else {
			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
			}

			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
			}

			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide3) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort3].RDMAEngine.ToOutside)
			}

			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
				}
			}
			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide3) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort3].RDMAEngine.ToOutside)
				}
			}
		}
	}
	return rdmaAddrTable
}

func (b *R9NanoPlatformBuilder) setCenterRDMAAddrTable(
	gpuCoodrinateSide1 []gpuCoodrinate,
	gpuCoodrinateSide2 []gpuCoodrinate,
	gpuCoodrinateSide3 []gpuCoodrinate,
	gpuCoodrinateSide4 []gpuCoodrinate,
	gpuPort1 int,
	gpuPort2 int,
	gpuPort3 int,
	gpuPort4 int,
	maxNumHops int, gpuLocationX int, gpuLocationY int,
) mem.AddressBoundary {
	rdmaAddrTable := mem.AddressBoundary{}
	gpuMatrix := b.gpuMatrix()
	for _, gpu := range gpuMatrix {
		assignedgpuID := gpu.gpuID
		x, y := b.convertGPUIDToCoodrinate(assignedgpuID)
		hops := int(math.Abs(float64(x-gpuLocationX) + math.Abs(float64(y-gpuLocationY))))
		if hops <= maxNumHops {
			rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
			rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
			rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[assignedgpuID].RDMAEngine.ToOutside)
		} else {
			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
			}

			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
			}

			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort3].RDMAEngine.ToOutside)
			}

			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
				rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort4].RDMAEngine.ToOutside)
			}

			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
				}
			}
			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort1].RDMAEngine.ToOutside)
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort4].RDMAEngine.ToOutside)
				}
			}
			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort2].RDMAEngine.ToOutside)
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort3].RDMAEngine.ToOutside)
				}
			}
			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
				b.isContain(assignedgpuID, gpuCoodrinateSide4) {
				rand := rand.Intn(2)
				if rand == 0 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort3].RDMAEngine.ToOutside)
				}
				if rand == 1 {
					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID)*mem.GB)
					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64(assignedgpuID+1)*mem.GB)
					rdmaAddrTable.Port = append(rdmaAddrTable.Port, b.gpus[gpuPort4].RDMAEngine.ToOutside)
				}
			}
		}
	}
	return rdmaAddrTable
}

func (b *R9NanoPlatformBuilder) setRDMAAddrTable(gpuLocationX int, gpuLocationY int, maxNumHops int) *mem.AddressBoundary {
	// var gpuLocationX, gpuLocationY int
	newrdmaAddrTable := b.createRDMAAddrTable()
	rdmaAddrTable := mem.AddressBoundary{}
	// gpuLocationX, gpuLocationY = b.convertGPUIDToCoodrinate(gpuID)
	upPort, downPort, leftPort, rightPort := b.sellectPortGPU(gpuLocationX, gpuLocationY, maxNumHops)
	gpuCoodrinateListUp, gpuCoodrinateListDown, gpuCoodrinateListLeft, gpuCoodrinateListRight := b.gpuMatrixPartation(gpuLocationX, gpuLocationY)
	if gpuLocationX == 0 && gpuLocationY == 0 {
		rdmaAddrTable = b.setConnerRDMATable(gpuCoodrinateListDown, gpuCoodrinateListRight, downPort, rightPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationX == b.tileWidth && gpuLocationY == 0 {
		rdmaAddrTable = b.setConnerRDMATable(gpuCoodrinateListDown, gpuCoodrinateListLeft, downPort, leftPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationX == 0 && gpuLocationY == b.tileHeight {
		rdmaAddrTable = b.setConnerRDMATable(gpuCoodrinateListUp, gpuCoodrinateListRight, upPort, rightPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationX == b.tileWidth && gpuLocationY == b.tileHeight {
		rdmaAddrTable = b.setConnerRDMATable(gpuCoodrinateListUp, gpuCoodrinateListLeft, upPort, leftPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationY == 0 && gpuLocationX != 0 && gpuLocationX != b.tileWidth {
		rdmaAddrTable = b.setSideRDMAAddrTable(gpuCoodrinateListLeft, gpuCoodrinateListDown, gpuCoodrinateListRight, leftPort, downPort, rightPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationY == b.tileHeight && gpuLocationX != 0 && gpuLocationX != b.tileWidth {
		rdmaAddrTable = b.setSideRDMAAddrTable(gpuCoodrinateListLeft, gpuCoodrinateListUp, gpuCoodrinateListRight, leftPort, upPort, rightPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationX == 0 && gpuLocationY != 0 && gpuLocationY != b.tileHeight {
		rdmaAddrTable = b.setSideRDMAAddrTable(gpuCoodrinateListUp, gpuCoodrinateListRight, gpuCoodrinateListDown, upPort, rightPort, downPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationX == b.tileWidth && gpuLocationY != 0 && gpuLocationY != b.tileHeight {
		rdmaAddrTable = b.setSideRDMAAddrTable(gpuCoodrinateListUp, gpuCoodrinateListLeft, gpuCoodrinateListDown, upPort, leftPort, downPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	if gpuLocationX != 0 && gpuLocationX != b.tileWidth && gpuLocationY != 0 && gpuLocationY != b.tileWidth {
		rdmaAddrTable = b.setCenterRDMAAddrTable(gpuCoodrinateListUp, gpuCoodrinateListRight, gpuCoodrinateListDown, gpuCoodrinateListLeft, upPort, rightPort, downPort, leftPort, maxNumHops, gpuLocationX, gpuLocationY)
	}
	newrdmaAddrTable.HighAddr = append(newrdmaAddrTable.HighAddr, rdmaAddrTable.HighAddr...)
	newrdmaAddrTable.LowAddr = append(newrdmaAddrTable.LowAddr, rdmaAddrTable.LowAddr...)
	newrdmaAddrTable.Port = append(newrdmaAddrTable.Port, rdmaAddrTable.Port...)
	return newrdmaAddrTable
}
