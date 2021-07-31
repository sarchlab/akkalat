package runner

import (
	"fmt"

	"gitlab.com/akita/akita/v2/monitoring"
	"gitlab.com/akita/akita/v2/sim"
	"gitlab.com/akita/mem/v2/dram"
	"gitlab.com/akita/mem/v2/mem"
	"gitlab.com/akita/mem/v2/vm/tlb"
	"gitlab.com/akita/mgpusim/v2/timing/cp"
	"gitlab.com/akita/mgpusim/v2/timing/pagemigrationcontroller"
	meshNetwork "gitlab.com/akita/noc/v2/networking/mesh"
	"gitlab.com/akita/util/v2/tracing"
)

type mesh struct {
	tiles      []*tile
	tilesPorts [][]sim.Port
	// drams              []*idealmemcontroller.Comp
	memLowModuleFinder *mem.InterleavedLowModuleFinder
	meshConn           *meshNetwork.Connector
}

type meshBuilder struct {
	gpuID   uint64
	gpuName string
	name    string
	gpuPtr  *GPU
	cp      *cp.CommandProcessor
	l2TLB   *tlb.TLB

	engine             sim.Engine
	freq               sim.Freq
	memAddrOffset      uint64
	log2CacheLineSize  uint64
	log2PageSize       uint64
	dramSize           uint64
	visTracer          tracing.Tracer
	memTracer          tracing.Tracer
	enableISADebugging bool
	enableMemTracing   bool
	enableVisTracing   bool
	monitor            *monitoring.Monitor

	tileWidth                      int
	tileHeight                     int
	numTile                        int
	numMemoryBank                  int
	log2MemoryBankInterleavingSize uint64

	dmaEngine               *cp.DMAEngine
	dramBuilder             dram.Builder
	globalStorage           *mem.Storage
	pageMigrationController *pagemigrationcontroller.PageMigrationController
}

func makeMeshBuilder() meshBuilder {
	b := meshBuilder{
		gpuID:      0,
		name:       "Mesh",
		freq:       1 * sim.GHz,
		tileWidth:  0,
		tileHeight: 0,
	}
	return b
}

func (b meshBuilder) withGPUID(id uint64) meshBuilder {
	b.gpuID = id
	return b
}

func (b meshBuilder) withGPUName(str string) meshBuilder {
	b.gpuName = str
	return b
}

func (b meshBuilder) withGPUPtr(p *GPU) meshBuilder {
	b.gpuPtr = p
	return b
}

func (b meshBuilder) withCP(cp *cp.CommandProcessor) meshBuilder {
	b.cp = cp
	return b
}

func (b meshBuilder) withL2TLB(tlb *tlb.TLB) meshBuilder {
	b.l2TLB = tlb
	return b
}

func (b meshBuilder) withEngine(e sim.Engine) meshBuilder {
	b.engine = e
	return b
}

func (b meshBuilder) withFreq(f sim.Freq) meshBuilder {
	b.freq = f
	return b
}

func (b meshBuilder) WithMemAddrOffset(offset uint64) meshBuilder {
	b.memAddrOffset = offset
	return b
}

func (b meshBuilder) withLog2CachelineSize(
	log2Size uint64,
) meshBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

func (b meshBuilder) withLog2PageSize(
	log2Size uint64,
) meshBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b meshBuilder) WithDRAMSize(size uint64) meshBuilder {
	b.dramSize = size
	return b
}

func (b meshBuilder) WithISADebugging() meshBuilder {
	b.enableISADebugging = true
	return b
}

func (b meshBuilder) withVisTracer(t tracing.Tracer) meshBuilder {
	b.enableVisTracing = true
	b.visTracer = t
	return b
}

func (b meshBuilder) withMemTracer(t tracing.Tracer) meshBuilder {
	b.enableMemTracing = true
	b.memTracer = t
	return b
}

func (b meshBuilder) withMonitor(monitor *monitoring.Monitor) meshBuilder {
	b.monitor = monitor
	return b
}

func (b meshBuilder) withTileWidth(w int) meshBuilder {
	b.tileWidth = w
	return b
}

func (b meshBuilder) withTileHeight(h int) meshBuilder {
	b.tileHeight = h
	return b
}

func (b meshBuilder) withLog2MemoryBankInterleavingSize(
	n uint64,
) meshBuilder {
	b.log2MemoryBankInterleavingSize = n
	return b
}

func (b meshBuilder) withDMAEngine(
	e *cp.DMAEngine,
) meshBuilder {
	b.dmaEngine = e
	return b
}

func (b meshBuilder) withPageMigrationController(
	pmc *pagemigrationcontroller.PageMigrationController,
) meshBuilder {
	b.pageMigrationController = pmc
	return b
}

func (b meshBuilder) withGlobalStorage(s *mem.Storage) meshBuilder {
	b.globalStorage = s
	return b
}

func (b *meshBuilder) separatePeripheralPorts(m *mesh, ports []sim.Port) {
	b.numTile = b.tileHeight * b.tileWidth
	if b.numTile <= 0 {
		errMsg := "Number of tile height and width must be set to positive " +
			"number before separating peripheral ports of the mesh to the edge!\n"
		panic(errMsg)
	}
	m.tilesPorts = make([][]sim.Port, b.numTile)
	m.tilesPorts[0] = append(m.tilesPorts[0], ports...)

	// batch := len(ports)
	// load := batch / b.tileWidth
	// rest := batch % b.tileWidth
	// for i := 0; i < b.tileWidth; i++ {
	// 	if i < rest {
	// 		startPort := i * (load + 1)
	// 		endPort := startPort + load + 1
	// 		for j := startPort; j < endPort; j++ {
	// 			m.tilesPorts[i] = append(m.tilesPorts[i], ports[j])
	// 		}
	// 	} else {
	// 		startPort := i*load + rest
	// 		endPort := startPort + load
	// 		for j := startPort; j < endPort; j++ {
	// 			m.tilesPorts[i] = append(m.tilesPorts[i], ports[j])
	// 		}
	// 	}
	// }
}

func (b *meshBuilder) Build(
	name string,
	periphPorts []sim.Port,
) *mesh {
	b.name = name
	b.numMemoryBank = b.tileHeight * b.tileWidth
	b.dramBuilder = b.createDramControllerBuilder()

	m := mesh{}
	b.separatePeripheralPorts(&m, periphPorts)

	m.meshConn = meshNetwork.NewConnector().
		WithEngine(b.engine).
		WithFreq(b.freq)
	m.meshConn.CreateNetwork(b.name)

	b.buildTiles(&m)

	m.meshConn.EstablishNetwork()

	return &m
}

func (b *meshBuilder) buildTiles(m *mesh) {
	tileBuilder := makeTileBuilder().
		withEngine(b.engine).
		withFreq(b.freq).
		withGPUID(b.gpuID).
		withLog2CachelineSize(b.log2CacheLineSize).
		withLog2PageSize(b.log2PageSize).
		withDRAMBuilder(b.dramBuilder)

	if b.enableISADebugging {
		tileBuilder = tileBuilder.withIsaDebugging()
	}

	if b.enableVisTracing {
		tileBuilder = tileBuilder.withVisTracer(b.visTracer)
	}

	if b.enableMemTracing {
		tileBuilder = tileBuilder.withMemTracer(b.memTracer)
	}

	for x := 0; x < b.tileHeight; x++ {
		for y := 0; y < b.tileWidth; y++ {
			name := fmt.Sprintf("%s.Tile_[%02d,%02d]", b.name, x, y)
			t := tileBuilder.Build(name)
			m.tiles = append(m.tiles, &t)
		}
	}

	b.buildMemoryLowModules(m)
	b.fillATsLowModules(m)
	b.fillL1TLBLowModules(m)
	b.exportTilesPorts(m)
	b.populateMeshComponents(m)

	for x := 0; x < b.tileHeight; x++ {
		for y := 0; y < b.tileWidth; y++ {
			idx := x*b.tileWidth + y
			m.meshConn.AddTile([3]int{x, y, 0}, m.tilesPorts[idx])
		}
	}
}

func (b *meshBuilder) buildMemoryLowModules(m *mesh) {
	// lowModuleFinder := mem.NewBankedLowModuleFinder(b.dramSize)
	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)

	// lowModuleFinder.UseAddressSpaceLimitation = true
	// lowModuleFinder.LowAddress = b.memAddrOffset
	// lowModuleFinder.HighAddress = b.memAddrOffset + b.dramSize

	for _, t := range m.tiles {
		lowModuleFinder.LowModules = append(
			lowModuleFinder.LowModules,
			t.mem.GetPortByName("Top"),
		)
	}

	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
	m.memLowModuleFinder = lowModuleFinder
}

func (b *meshBuilder) createDramControllerBuilder() dram.Builder {
	memBankSize := b.dramSize / uint64(b.numMemoryBank)
	if b.dramSize%uint64(b.numMemoryBank) != 0 {
		panic("GPU memory size is not a multiple of the number of memory banks")
	}

	dramCol := 64
	dramRow := 16384
	dramDeviceWidth := 128
	dramBankSize := dramCol * dramRow * dramDeviceWidth
	dramBank := 4
	dramBankGroup := 4
	dramBusWidth := 256
	dramDevicePerRank := dramBusWidth / dramDeviceWidth
	dramRankSize := dramBankSize * dramDevicePerRank * dramBank
	dramRank := int(memBankSize * 8 / uint64(dramRankSize))

	memCtrlBuilder := dram.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(500 * sim.MHz).
		WithProtocol(dram.HBM).
		WithBurstLength(4).
		WithDeviceWidth(dramDeviceWidth).
		WithBusWidth(dramBusWidth).
		WithNumChannel(1).
		WithNumRank(dramRank).
		WithNumBankGroup(dramBankGroup).
		WithNumBank(dramBank).
		WithNumCol(dramCol).
		WithNumRow(dramRow).
		WithCommandQueueSize(8).
		WithTransactionQueueSize(32).
		WithTCL(7).
		WithTCWL(2).
		WithTRCDRD(7).
		WithTRCDWR(7).
		WithTRP(7).
		WithTRAS(17).
		WithTREFI(1950).
		WithTRRDS(2).
		WithTRRDL(3).
		WithTWTRS(3).
		WithTWTRL(4).
		WithTWR(8).
		WithTCCDS(1).
		WithTCCDL(1).
		WithTRTRS(0).
		WithTRTP(3).
		WithTPPD(2)

	if b.visTracer != nil {
		memCtrlBuilder = memCtrlBuilder.WithAdditionalTracer(b.visTracer)
	}

	if b.globalStorage != nil {
		memCtrlBuilder = memCtrlBuilder.WithGlobalStorage(b.globalStorage)
	}

	return memCtrlBuilder
}

func (b *meshBuilder) exportTilesPorts(m *mesh) {
	b.exportCUs(m)
	b.exportROBs(m)
	b.exportATs(m)
	b.exportDRAMs(m)
	b.exportL1TLBs(m)
}

func (b *meshBuilder) exportCUs(m *mesh) {
	/* CUs(ToACE) <-> CP */
	/* CUs(ToCP) <-> CP ; ToCP is control port of CU */
	for idx, t := range m.tiles {
		cu := t.cu
		b.cp.RegisterCU(cu)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], cu.ToACE)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], cu.ToCP)
	}
}

func (b *meshBuilder) exportROBs(m *mesh) {
	var ctrlPort sim.Port
	for idx, t := range m.tiles {
		/* L1vROB(Control) <-> CP */
		ctrlPort = t.l1vROB.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)

		/* L1sROB(Control) <-> CP */
		ctrlPort = t.l1sROB.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)

		/* L1iROB(Control) <-> CP */
		ctrlPort = t.l1iROB.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
	}
}

func (b *meshBuilder) exportATs(m *mesh) {
	var ctrlPort, bottom sim.Port
	for idx, t := range m.tiles {
		/* L1vAT(Bottom) <-> Mesh Memory(Top) */
		/* L1vAT(Control) <-> CP */
		bottom = t.l1vAT.GetPortByName("Bottom")
		ctrlPort = t.l1vAT.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)

		/* L1sAT(Bottom) <-> Mesh Memory(Top) */
		/* L1sAT(Control) <-> CP */
		bottom = t.l1sAT.GetPortByName("Bottom")
		ctrlPort = t.l1sAT.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)

		/* L1iAT(Bottom) <-> Mesh Memory(Top) */
		/* L1iAT(Control) <-> CP */
		bottom = t.l1iAT.GetPortByName("Bottom")
		ctrlPort = t.l1iAT.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
	}
}

func (b *meshBuilder) exportDRAMs(m *mesh) {
	var top sim.Port
	for idx, t := range m.tiles {
		/* Mesh Memory(Top) <-> L1vAT(Bottom) */
		/* Mesh Memory(Top) <-> L1sAT(Bottom) */
		/* Mesh Memory(Top) <-> L1iAT(Bottom) */
		top = t.mem.GetPortByName("Top")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], top)
	}
}

func (b *meshBuilder) exportL1TLBs(m *mesh) {
	var bottom, ctrlPort sim.Port
	for idx, t := range m.tiles {
		/* L1vTLBs(Bottom) <-> L2TLB(Top) */
		/* L1vTLBs(Control) <-> CP */
		bottom = t.l1vTLB.GetPortByName("Bottom")
		ctrlPort = t.l1vTLB.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)

		/* L1sTLB(Bottom) <-> L2TLB(Top) */
		/* L1sTLB(Control) <-> CP */
		bottom = t.l1sTLB.GetPortByName("Bottom")
		ctrlPort = t.l1sTLB.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)

		/* L1iTLB(Bottom) <-> L2TLB(Top) */
		/* L1iTLB(Control) <-> CP */
		bottom = t.l1iTLB.GetPortByName("Bottom")
		ctrlPort = t.l1iTLB.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
	}
}

func (b *meshBuilder) populateMeshComponents(m *mesh) {
	// Only populate components in tile, since the
	// L2Caches have been populated during the build stage.
	b.populateCUs(m)
	b.populateROBs(m)
	b.populateATs(m)
	b.populateDRAMs(m)
	b.populateL1TLBs(m)
}

func (b *meshBuilder) populateCUs(m *mesh) {
	for _, t := range m.tiles {
		b.gpuPtr.CUs = append(b.gpuPtr.CUs, t.cu)

		if b.monitor != nil {
			b.monitor.RegisterComponent(t.cu)
		}
	}
}

func (b *meshBuilder) populateROBs(m *mesh) {
	for _, t := range m.tiles {
		if b.monitor != nil {
			b.monitor.RegisterComponent(t.l1vROB)
		}
	}
}

func (b *meshBuilder) populateATs(m *mesh) {
	for _, t := range m.tiles {
		if b.monitor != nil {
			b.monitor.RegisterComponent(t.l1vAT)
		}
	}
}

func (b *meshBuilder) populateL1TLBs(m *mesh) {
	for _, t := range m.tiles {
		b.gpuPtr.L1VTLBs = append(b.gpuPtr.L1VTLBs, t.l1vTLB)
		b.gpuPtr.L1STLBs = append(b.gpuPtr.L1STLBs, t.l1sTLB)
		b.gpuPtr.L1ITLBs = append(b.gpuPtr.L1ITLBs, t.l1iTLB)

		if b.monitor != nil {
			b.monitor.RegisterComponent(t.l1vTLB)
		}
	}
}

func (b *meshBuilder) populateDRAMs(m *mesh) {
	for _, t := range m.tiles {
		b.gpuPtr.MemControllers = append(b.gpuPtr.MemControllers, t.mem)
	}
}

func (b *meshBuilder) fillL1TLBLowModules(m *mesh) {
	for _, t := range m.tiles {
		t.l1vTLB.LowModule = b.l2TLB.GetPortByName("Top")
		t.l1iTLB.LowModule = b.l2TLB.GetPortByName("Top")
		t.l1sTLB.LowModule = b.l2TLB.GetPortByName("Top")
	}
}

func (b *meshBuilder) fillATsLowModules(m *mesh) {
	for _, t := range m.tiles {
		t.l1vAT.SetLowModuleFinder(m.memLowModuleFinder)
		t.l1sAT.SetLowModuleFinder(m.memLowModuleFinder)
		t.l1iAT.SetLowModuleFinder(m.memLowModuleFinder)
	}
}
