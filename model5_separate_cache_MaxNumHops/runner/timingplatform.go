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
func (b R9NanoPlatformBuilder) Build(maxNumHops int) *Platform {
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
		rdmaAddressTable, pmcAddressTable, maxNumHops)

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

type addressMappingEntry struct {
	gpuID             int
	x, y              int
	lowAddr, highAddr uint64
	port              sim.Port
}

func (b *R9NanoPlatformBuilder) createGPUs(
	connector *mesh.Connector,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.GenericLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
	maxNumHops int,
) {
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}
			b.createGPU(x, y,
				gpuBuilder, gpuDriver,
				rdmaAddressTable, pmcAddressTable,
				connector,
			)
		}
	}

	// Create a map for all the GPUs
	entries := make([]addressMappingEntry, 0, len(b.gpus)+1)
	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			entry := &addressMappingEntry{
				x: x, y: y,
			}

			gpuIndex := b.convertCoordinateToGPUID(x, y)
			if gpuIndex == 0 {
				continue
			} else {
				entry.gpuID = gpuIndex
				entry.lowAddr = uint64(gpuIndex) * 4 * mem.GB
				entry.highAddr = entry.lowAddr + 4*mem.GB
				// portName := "GPU_" + strconv.Itoa(x) + "_" + strconv.Itoa(y) + ".RDMA.ToOutside"
				entry.port = b.gpus[gpuIndex-1].RDMAEngine.GetPortByName("ToOutside")

				entries = append(entries, *entry)
			}
		}
	}

	for y := 0; y < b.tileHeight; y++ {
		for x := 0; x < b.tileWidth; x++ {
			if x == b.tileWidth/2 && y == b.tileHeight/2 {
				continue
			}
			gpuID := b.convertCoordinateToGPUID(x, y)
			b.gpus[gpuID-1].RDMAEngine.RemoteRDMAAddressTable =
				b.setRDMAAddrTable(x, y, maxNumHops, entries)
		}
	}
}

func (b *R9NanoPlatformBuilder) setRDMAAddrTable(
	x, y, maxNumHops int,
	entries []addressMappingEntry,
) *mem.GenericLowModuleFinder {
	rdmaAddressTable := new(mem.GenericLowModuleFinder)

	for _, entry := range entries {
		// if entry.x == x && entry.y == y {
		// 	continue
		// }

		if abs(x-entry.x)+abs(y-entry.y) > maxNumHops {
			// Find all coordinates that are within maxNumHops hops
			// from the current GPU
			candidates := b.getCandidates(x, y, maxNumHops, entries)

			// Find the closest candidates
			lowestDist := math.MaxInt
			for _, candidate := range candidates {
				dist := abs(entry.x-candidate.x) + abs(entry.y-candidate.y)
				if dist < lowestDist {
					lowestDist = dist
				}
			}

			// Filter out all candidates that are not the closest
			filteredCandidate := make([]addressMappingEntry, 0, len(candidates))
			for _, candidate := range candidates {
				dist := abs(entry.x-candidate.x) + abs(entry.y-candidate.y)
				if dist == lowestDist {
					filteredCandidate = append(filteredCandidate, candidate)
				}
			}

			// Randomly pick one of the closest candidates
			chosen := filteredCandidate[rand.Intn(len(filteredCandidate))]

			// Add the chosen entry to the RDMA address table
			rdmaAddressTable.AddAddressRange(
				entry.lowAddr, entry.highAddr, chosen.port)
		} else {
			rdmaAddressTable.AddAddressRange(
				entry.lowAddr, entry.highAddr, entry.port)
		}
	}

	return rdmaAddressTable
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (b R9NanoPlatformBuilder) createPMCPageTable() *mem.BankedLowModuleFinder {
	pmcAddressTable := new(mem.BankedLowModuleFinder)
	pmcAddressTable.BankSize = 4 * mem.GB
	pmcAddressTable.LowModules = append(pmcAddressTable.LowModules, nil)
	return pmcAddressTable
}

func (b R9NanoPlatformBuilder) createRDMAAddrTable() *mem.GenericLowModuleFinder {
	rdmaAddressTable := new(mem.GenericLowModuleFinder)

	rdmaAddressTable.AddAddressRange(0, 4*mem.GB, nil)

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
	rdmaAddressTable *mem.GenericLowModuleFinder,
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
// 	addrTable *mem.BankedLowModuleFinder,
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
	cpuLocation := ((b.tileWidth)*(b.tileHeight) - 1) / 2
	if gpuID <= cpuLocation {
		y = (gpuID - 1) / (b.tileWidth)
		x = (gpuID - 1) % (b.tileWidth)
	} else {
		y = gpuID / (b.tileWidth)
		x = gpuID % (b.tileWidth)
	}
	return x, y
}

func (b *R9NanoPlatformBuilder) convertCoordinateToGPUID(x int, y int) int {
	var gpuID int
	if y < b.tileHeight/2 {
		gpuID = y*(b.tileWidth) + x + 1
	}
	if y == b.tileHeight/2 && x < b.tileWidth/2 {
		gpuID = y*(b.tileWidth) + x + 1
	}
	if y == b.tileHeight/2 && x > b.tileWidth/2 {
		gpuID = y*(b.tileWidth) + x
	}
	if y > b.tileHeight/2 {
		gpuID = y*(b.tileWidth) + x
	}
	return gpuID
}

// type gpuCoodrinate struct {
// 	gpuID int
// 	x     int
// 	y     int
// }

// func NewGPUCoodrinate(gpuID int, x int, y int) *gpuCoodrinate {
// 	return &gpuCoodrinate{
// 		gpuID: gpuID,
// 		x:     x,
// 		y:     y,
// 	}
// }
// func (b *R9NanoPlatformBuilder) gpuMatrix() []gpuCoodrinate {
// 	var gpuID int = 1
// 	gpuCoodrinateList := []gpuCoodrinate{}
// 	newGPUCoodrinate := NewGPUCoodrinate
// 	for y := 0; y < b.tileHeight; y++ {
// 		for x := 0; x < b.tileWidth; x++ {
// 			if x == b.tileWidth/2 && y == b.tileHeight/2 {
// 				continue
// 			}
// 			gpuCoodrinateList = append(gpuCoodrinateList, *newGPUCoodrinate(gpuID, x, y))
// 			gpuID = gpuID + 1
// 		}
// 	}
// 	return gpuCoodrinateList
// }

// func (b *R9NanoPlatformBuilder) gpuMatrixPartition(gpuLocationX int, gpuLocationY int) ([]gpuCoodrinate, []gpuCoodrinate, []gpuCoodrinate, []gpuCoodrinate) {
// 	var gpuID int = 1
// 	gpuCoodrinateListUp := []gpuCoodrinate{}
// 	gpuCoodrinateListDown := []gpuCoodrinate{}
// 	gpuCoodrinateListLeft := []gpuCoodrinate{}
// 	gpuCoodrinateListRight := []gpuCoodrinate{}
// 	newGPUCoodrinate := NewGPUCoodrinate
// 	for y := 0; y < b.tileHeight; y++ {
// 		for x := 0; x < b.tileWidth; x++ {
// 			if x == b.tileWidth/2 && y == b.tileHeight/2 {
// 				continue
// 			}
// 			if y < gpuLocationY {
// 				gpuCoodrinateListUp = append(gpuCoodrinateListUp, *newGPUCoodrinate(gpuID, x, y))
// 			}
// 			if y > gpuLocationY {
// 				gpuCoodrinateListDown = append(gpuCoodrinateListDown, *newGPUCoodrinate(gpuID, x, y))
// 			}
// 			if x < gpuLocationX {
// 				gpuCoodrinateListLeft = append(gpuCoodrinateListLeft, *newGPUCoodrinate(gpuID, x, y))
// 			}
// 			if x > gpuLocationX {
// 				gpuCoodrinateListRight = append(gpuCoodrinateListRight, *newGPUCoodrinate(gpuID, x, y))
// 			}
// 			gpuID = gpuID + 1
// 		}
// 	}
// 	return gpuCoodrinateListUp, gpuCoodrinateListDown, gpuCoodrinateListLeft, gpuCoodrinateListRight
// }

func (b *R9NanoPlatformBuilder) getCandidates(
	gpuX, gpuY int,
	maxNumHops int,
	gpuCoordinateList []addressMappingEntry,
) []addressMappingEntry {
	candidates := make([]addressMappingEntry, 0)

	for _, coordinate := range gpuCoordinateList {
		x := coordinate.x
		y := coordinate.y
		hops := abs(x-gpuX) + abs(y-gpuY)
		if hops == maxNumHops {
			candidates = append(candidates, coordinate)
		}
	}

	return candidates
}

// func (b *R9NanoPlatformBuilder) selectPortGPU(gpuLocationX int, gpuLocationY int, maxNumHops int) (int, int, int, int) {
// 	var upPort, downPort, leftPort, rightPort = 0, 0, 0, 0

// 	gpuCoodrinateListUp, gpuCoodrinateListDown, gpuCoodrinateListLeft, gpuCoodrinateListRight := b.gpuMatrixPartation(gpuLocationX, gpuLocationY)
// 	if gpuLocationX == 0 && gpuLocationY == 0 {
// 		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
// 		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
// 		for _, downPorts := range downPortCandidate {
// 			if downPorts == 0 {
// 				continue
// 			} else {
// 				downPort = downPorts
// 			}
// 		}
// 		for _, rightPorts := range rightPortCandidate {
// 			if rightPorts == downPort {
// 				continue
// 			} else if rightPorts == 0 {
// 				continue
// 			} else {
// 				rightPort = rightPorts
// 				break
// 			}
// 		}
// 	}

// 	if gpuLocationX == (b.tileWidth-1) && gpuLocationY == 0 {
// 		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
// 		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
// 		for _, downPorts := range downPortCandidate {
// 			if downPorts == 0 {
// 				continue
// 			} else {
// 				downPort = downPorts
// 			}
// 		}
// 		for _, leftPorts := range leftPortCandidate {
// 			if leftPorts == downPort {
// 				continue
// 			} else if leftPorts == 0 {
// 				continue
// 			} else {
// 				leftPort = leftPorts
// 				break
// 			}
// 		}
// 	}

// 	if gpuLocationX == 0 && gpuLocationY == (b.tileHeight-1) {
// 		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
// 		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
// 		upPort = upPortCandidate[0]
// 		for _, upPorts := range upPortCandidate {
// 			if upPorts == 0 {
// 				continue
// 			} else {
// 				upPort = upPorts
// 			}
// 		}
// 		for _, rightPorts := range rightPortCandidate {
// 			if rightPorts == upPort {
// 				continue
// 			} else if rightPorts == 0 {
// 				continue
// 			} else {
// 				rightPort = rightPorts
// 				break
// 			}
// 		}
// 	}

// 	if gpuLocationX == (b.tileWidth-1) && gpuLocationY == (b.tileHeight-1) {
// 		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
// 		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
// 		upPort = upPortCandidate[0]
// 		for _, upPorts := range upPortCandidate {
// 			if upPorts == 0 {
// 				continue
// 			} else {
// 				upPort = upPorts
// 			}
// 		}
// 		for _, leftPorts := range leftPortCandidate {
// 			if leftPorts == upPort {
// 				continue
// 			} else if leftPorts == 0 {
// 				continue
// 			} else {
// 				leftPort = leftPorts
// 				break
// 			}
// 		}
// 	}

// 	if gpuLocationX != 0 && gpuLocationX != (b.tileWidth-1) && gpuLocationY == 0 {
// 		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
// 		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
// 		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
// 		for _, leftPorts := range leftPortCandidate {
// 			if leftPorts == 0 {
// 				continue
// 			} else {
// 				leftPort = leftPorts
// 			}
// 		}
// 		for _, rightPorts := range rightPortCandidate {
// 			if rightPorts == leftPort {
// 				continue
// 			} else if rightPorts == 0 {
// 				continue
// 			} else {
// 				rightPort = rightPorts
// 				break
// 			}
// 		}

// 		for _, downPorts := range downPortCandidate {
// 			if downPorts == leftPort || downPorts == rightPort {
// 				continue
// 			} else if downPorts == 0 {
// 				continue
// 			} else {
// 				downPort = downPorts
// 				break
// 			}
// 		}
// 		if leftPort == 0 {
// 			leftPort = downPort
// 		}
// 		if rightPort == 0 {
// 			rightPort = downPort
// 		}
// 		if downPort == 0 {
// 			downPort = leftPort
// 		}
// 	}

// 	if gpuLocationX != 0 && gpuLocationX != (b.tileWidth-1) && gpuLocationY == (b.tileHeight-1) {
// 		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
// 		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
// 		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
// 		for _, leftPorts := range leftPortCandidate {
// 			if leftPorts == 0 {
// 				continue
// 			} else {
// 				leftPort = leftPorts
// 			}
// 		}
// 		for _, rightPorts := range rightPortCandidate {
// 			if rightPorts == leftPort {
// 				continue
// 			} else if rightPorts == 0 {
// 				continue
// 			} else {
// 				rightPort = rightPorts
// 				break
// 			}
// 		}

// 		for _, upPorts := range upPortCandidate {
// 			if upPorts == leftPort || upPorts == rightPort {
// 				continue
// 			} else if upPorts == 0 {
// 				continue
// 			} else {
// 				upPort = upPorts
// 				break
// 			}
// 		}
// 		if leftPort == 0 {
// 			leftPort = upPort
// 		}
// 		if rightPort == 0 {
// 			rightPort = upPort
// 		}
// 		if upPort == 0 {
// 			upPort = leftPort
// 		}
// 	}

// 	if gpuLocationY != 0 && gpuLocationY != (b.tileHeight-1) && gpuLocationX == 0 {
// 		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
// 		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
// 		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
// 		upPort = upPortCandidate[0]
// 		for _, upPorts := range upPortCandidate {
// 			if upPorts == 0 {
// 				continue
// 			} else {
// 				upPort = upPorts
// 			}
// 		}
// 		for _, downPorts := range downPortCandidate {
// 			if downPorts == leftPort {
// 				continue
// 			} else if downPorts == 0 {
// 				continue
// 			} else {
// 				downPort = downPorts
// 				break
// 			}
// 		}

// 		for _, rightPorts := range rightPortCandidate {
// 			if rightPorts == upPort || rightPorts == downPort {
// 				continue
// 			} else if rightPorts == 0 {
// 				continue
// 			} else {
// 				rightPort = rightPorts
// 				break
// 			}
// 		}
// 		if upPort == 0 {
// 			upPort = rightPort
// 		}
// 		if rightPort == 0 {
// 			leftPort = upPort
// 		}
// 		if downPort == 0 {
// 			downPort = rightPort
// 		}
// 	}

// 	if gpuLocationY != 0 && gpuLocationY != (b.tileHeight-1) && gpuLocationX == (b.tileWidth-1) {
// 		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
// 		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
// 		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
// 		upPort = upPortCandidate[0]
// 		for _, upPorts := range upPortCandidate {
// 			if upPorts == 0 {
// 				continue
// 			} else {
// 				upPort = upPorts
// 			}
// 		}
// 		for _, downPorts := range downPortCandidate {
// 			if downPorts == leftPort {
// 				continue
// 			} else if downPorts == 0 {
// 				continue
// 			} else {
// 				downPort = downPorts
// 				break
// 			}
// 		}

// 		for _, leftPorts := range leftPortCandidate {
// 			if leftPorts == upPort || leftPorts == downPort {
// 				continue
// 			} else if leftPorts == 0 {
// 				continue
// 			} else {
// 				leftPort = leftPorts
// 				break
// 			}
// 		}
// 		if upPort == 0 {
// 			upPort = leftPort
// 		}
// 		if leftPort == 0 {
// 			leftPort = upPort
// 		}
// 		if downPort == 0 {
// 			downPort = leftPort
// 		}
// 	}

// 	if gpuLocationY != 0 && gpuLocationY != (b.tileHeight-1) && gpuLocationX != (b.tileWidth-1) && gpuLocationX != 0 {
// 		upPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListUp)
// 		downPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListDown)
// 		leftPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListLeft)
// 		rightPortCandidate := b.getCandidates(gpuLocationX, gpuLocationY, maxNumHops, gpuCoodrinateListRight)
// 		// upPort = upPortCandidate[0]
// 		for _, upPorts := range upPortCandidate {
// 			if upPorts == 0 {
// 				continue
// 			} else {
// 				upPort = upPorts
// 			}
// 		}
// 		for _, downPorts := range downPortCandidate {
// 			if downPorts == leftPort {
// 				continue
// 			} else if downPorts == 0 {
// 				continue
// 			} else {
// 				downPort = downPorts
// 				break
// 			}
// 		}

// 		for _, leftPorts := range leftPortCandidate {
// 			if leftPorts == upPort || leftPorts == downPort {
// 				continue
// 			} else if leftPorts == 0 {
// 				continue
// 			} else {
// 				leftPort = leftPorts
// 				break
// 			}
// 		}

// 		for _, rightPorts := range rightPortCandidate {
// 			if rightPorts == upPort || rightPorts == downPort || rightPorts == leftPort {
// 				continue
// 			} else if rightPorts == 0 {
// 				continue
// 			} else {
// 				rightPort = rightPorts
// 				break
// 			}
// 		}

// 		if upPort == 0 {
// 			upPort = rightPort
// 		}
// 		if rightPort == 0 {
// 			rightPort = upPort
// 		}
// 		if downPort == 0 {
// 			downPort = leftPort
// 		}
// 		if leftPort == 0 {
// 			leftPort = downPort
// 		}
// 	}

// 	return upPort, downPort, leftPort, rightPort
// }

// func (b *R9NanoPlatformBuilder) isContain(gpuID int, gpuCoodrinateList []gpuCoodrinate) bool {
// 	for _, gpu := range gpuCoodrinateList {
// 		if gpu.gpuID == gpuID {
// 			return true
// 		}
// 	}
// 	return false
// }

// func (b *R9NanoPlatformBuilder) setConnerRDMATable(
// 	gpuCoodrinateSide1 []gpuCoodrinate,
// 	gpuCoodrinateSide2 []gpuCoodrinate,
// 	gpuPort1 int, gpuPort2 int, maxNumHops int, gpuLocationX int, gpuLocationY int) mem.GenericLowModuleFinder {
// 	rdmaAddrTable := mem.GenericLowModuleFinder{}
// 	// newrdmaAddrTable := mem.NewAddressBoundary
// 	gpuMatrix := b.gpuMatrix()
// 	for i, gpu := range gpuMatrix {
// 		assignedgpuID := gpu.gpuID
// 		x, y := b.convertGPUIDToCoodrinate(assignedgpuID)
// 		hops := int(math.Abs(float64(x-gpuLocationX) + math.Abs(float64(y-gpuLocationY))))
// 		if hops <= maxNumHops {
// 			rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 			rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 			rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[i].RDMAEngine.ToOutside)
// 		} else {
// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) && !b.isContain(assignedgpuID, gpuCoodrinateSide2) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 				continue
// 			}

// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) && b.isContain(assignedgpuID, gpuCoodrinateSide2) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 				continue
// 			}

// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) && b.isContain(assignedgpuID, gpuCoodrinateSide2) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 					continue
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 					continue
// 				}
// 			}
// 		}
// 	}
// 	return rdmaAddrTable
// }

// func (b *R9NanoPlatformBuilder) setSideRDMAAddrTable(
// 	gpuCoodrinateSide1 []gpuCoodrinate,
// 	gpuCoodrinateSide2 []gpuCoodrinate,
// 	gpuCoodrinateSide3 []gpuCoodrinate,
// 	gpuPort1 int,
// 	gpuPort2 int,
// 	gpuPort3 int,
// 	maxNumHops int, gpuLocationX int, gpuLocationY int,
// ) mem.GenericLowModuleFinder {
// 	rdmaAddrTable := mem.AddressBoundary{}
// 	gpuMatrix := b.gpuMatrix()
// 	for i, gpu := range gpuMatrix {
// 		assignedgpuID := gpu.gpuID
// 		x, y := b.convertGPUIDToCoodrinate(assignedgpuID)
// 		hops := int(math.Abs(float64(x-gpuLocationX) + math.Abs(float64(y-gpuLocationY))))
// 		if hops <= maxNumHops {
// 			rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 			rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 			rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[i].RDMAEngine.ToOutside)
// 		} else {
// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 			}

// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 			}

// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide3) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort3-1].RDMAEngine.ToOutside)
// 			}

// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 				}
// 			}
// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide3) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort3-1].RDMAEngine.ToOutside)
// 				}
// 			}
// 		}
// 	}
// 	return rdmaAddrTable
// }

// func (b *R9NanoPlatformBuilder) setCenterRDMAAddrTable(
// 	gpuCoodrinateSide1 []gpuCoodrinate,
// 	gpuCoodrinateSide2 []gpuCoodrinate,
// 	gpuCoodrinateSide3 []gpuCoodrinate,
// 	gpuCoodrinateSide4 []gpuCoodrinate,
// 	gpuPort1 int,
// 	gpuPort2 int,
// 	gpuPort3 int,
// 	gpuPort4 int,
// 	maxNumHops int, gpuLocationX int, gpuLocationY int,
// ) mem.GenericLowModuleFinder {
// 	rdmaAddrTable := mem.AddressBoundary{}
// 	gpuMatrix := b.gpuMatrix()
// 	for i, gpu := range gpuMatrix {
// 		assignedgpuID := gpu.gpuID
// 		x, y := b.convertGPUIDToCoodrinate(assignedgpuID)
// 		hops := int(math.Abs(float64(x-gpuLocationX))) + int(math.Abs(float64(y-gpuLocationY)))
// 		if hops <= maxNumHops {
// 			rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 			rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 			rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[i].RDMAEngine.ToOutside)
// 		} else {
// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 			}

// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 			}

// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort3-1].RDMAEngine.ToOutside)
// 			}

// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 				rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 				rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort4-1].RDMAEngine.ToOutside)
// 			}

// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 				}
// 			}
// 			if b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort1-1].RDMAEngine.ToOutside)
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort4-1].RDMAEngine.ToOutside)
// 				}
// 			}
// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort2-1].RDMAEngine.ToOutside)
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort3-1].RDMAEngine.ToOutside)
// 				}
// 			}
// 			if !b.isContain(assignedgpuID, gpuCoodrinateSide1) &&
// 				!b.isContain(assignedgpuID, gpuCoodrinateSide2) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide3) &&
// 				b.isContain(assignedgpuID, gpuCoodrinateSide4) {
// 				rand := rand.Intn(2)
// 				if rand == 0 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort3-1].RDMAEngine.ToOutside)
// 				}
// 				if rand == 1 {
// 					rdmaAddrTable.LowAddr = append(rdmaAddrTable.LowAddr, uint64(assignedgpuID*4)*mem.GB)
// 					rdmaAddrTable.HighAddr = append(rdmaAddrTable.HighAddr, uint64((assignedgpuID+1)*4)*mem.GB)
// 					rdmaAddrTable.LowModules = append(rdmaAddrTable.LowModules, b.gpus[gpuPort4-1].RDMAEngine.ToOutside)
// 				}
// 			}
// 		}
// 	}
// 	return rdmaAddrTable
// }
