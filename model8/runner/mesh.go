package runner

import (
	"fmt"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/sim/bottleneckanalysis"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/timing/cp"
	"gitlab.com/akita/mgpusim/v3/timing/pagemigrationcontroller"
	meshNetwork "gitlab.com/akita/noc/v3/networking/mesh"
)

type portBucket = []sim.Port

type mesh struct {
	tiles              []*tile
	tilesPorts         []portBucket
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
	visTracer          tracing.Tracer
	nocTracer          tracing.Tracer
	memTracer          tracing.Tracer
	enableISADebugging bool
	enableVisTracing   bool
	enableNoCTracing   bool
	enableMemTracing   bool
	monitor            *monitoring.Monitor
	bufferAnalyzer     *bottleneckanalysis.BufferAnalyzer

	tileWidth                      int
	tileHeight                     int
	numTile                        int
	log2MemoryBankInterleavingSize uint64

	dmaEngine               *cp.DMAEngine
	globalStorage           *mem.Storage
	pageMigrationController *pagemigrationcontroller.PageMigrationController
}

func makeMeshBuilder() meshBuilder {
	b := meshBuilder{
		gpuID:      0,
		name:       "Mesh",
		freq:       1 * sim.GHz,
		tileWidth:  16,
		tileHeight: 16,
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

func (b meshBuilder) WithISADebugging() meshBuilder {
	b.enableISADebugging = true
	return b
}

func (b meshBuilder) WithVisTracer(t tracing.Tracer) meshBuilder {
	b.enableVisTracing = true
	b.visTracer = t
	return b
}

func (b meshBuilder) WithNoCTracer(t tracing.Tracer) meshBuilder {
	b.enableNoCTracing = true
	b.nocTracer = t
	return b
}

func (b meshBuilder) WithMemTracer(t tracing.Tracer) meshBuilder {
	b.enableMemTracing = true
	b.memTracer = t
	return b
}

func (b meshBuilder) WithMonitor(monitor *monitoring.Monitor) meshBuilder {
	b.monitor = monitor
	return b
}

func (b meshBuilder) WithBufferAnalyzer(
	analyzer *bottleneckanalysis.BufferAnalyzer,
) meshBuilder {
	b.bufferAnalyzer = analyzer
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

func blancePortsIntoBuckets(ports []sim.Port, buckets []*portBucket) {
	batch := len(ports)
	numBuckets := len(buckets)

	load := batch / numBuckets
	rest := batch % numBuckets

	for i := 0; i < numBuckets; i++ {
		var startPort, endPort int

		if i < rest {
			startPort = i * (load + 1)
			endPort = startPort + load + 1
		} else {
			startPort = i*load + rest
			endPort = startPort + load
		}

		for j := startPort; j < endPort; j++ {
			*buckets[i] = append(*buckets[i], ports[j])
		}
	}
}

func (b *meshBuilder) separatePeripheralPorts(m *mesh, ports []sim.Port) {
	b.numTile = b.tileHeight * b.tileWidth
	if b.numTile <= 0 {
		errMsg := "Number of tile height and width must be set to positive " +
			"number before separating peripheral ports of the mesh to the edge!\n"
		panic(errMsg)
	}

	// Simplest way, to attach peripheral ports to Tile[0, 0].
	// m.tilesPorts[0] = append(m.tilesPorts[0], ports...)

	m.tilesPorts = make([]portBucket, b.numTile)

	var buckets []*portBucket
	for i := 0; i < b.tileWidth; i++ {
		// North edge
		buckets = append(buckets, &m.tilesPorts[i])
		// South edge
		buckets = append(buckets, &m.tilesPorts[b.numTile-i-1])
	}
	for i := 1; i < b.tileHeight-1; i++ {
		// West edge (exclude ends of north line)
		buckets = append(buckets, &m.tilesPorts[i*b.tileWidth])
		// East edge (exclude ends of south line)
		buckets = append(buckets, &m.tilesPorts[i*b.tileWidth+b.tileWidth-1])
	}

	blancePortsIntoBuckets(ports, buckets)
}

func (b *meshBuilder) Build(
	name string,
	periphPorts []sim.Port,
) *mesh {
	b.name = name

	m := mesh{}
	b.separatePeripheralPorts(&m, periphPorts)

	m.meshConn = meshNetwork.NewConnector().
		WithEngine(b.engine).
		WithFreq(b.freq)

	if b.enableVisTracing {
		m.meshConn = m.meshConn.WithVisTracer(b.visTracer)
	}

	if b.enableNoCTracing {
		m.meshConn = m.meshConn.WithNoCTracer(b.nocTracer)
	}

	if b.bufferAnalyzer != nil {
		m.meshConn = m.meshConn.WithBufferAnalyzer(b.bufferAnalyzer)
	}

	m.meshConn.CreateNetwork(b.name)

	b.buildTiles(&m)

	m.meshConn.EstablishNetwork()

	return &m
}

func (b *meshBuilder) buildTiles(m *mesh) {
	decoder := insts.NewDisassembler()
	tileBuilder := makeTileBuilder().
		withEngine(b.engine).
		withFreq(b.freq).
		withGPUID(b.gpuID).
		withDecoder(decoder).
		withLog2CachelineSize(b.log2CacheLineSize).
		withLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage)

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
			// We could not use Tile_[x,y] here, because the comma is recognized as
			// a column separator in CSV format.
			name := fmt.Sprintf("%s.Tile[%02d][%02d]", b.name, x, y)
			t := tileBuilder.Build(name)
			m.tiles = append(m.tiles, &t)
		}
	}

	b.buildMemoryLowModules(m)
	b.fillATsAndCachesLowModules(m)
	b.fillL1TLBLowModules(m)
	b.exportTilesPorts(m)
	b.populateMeshComponents(m)

	for x := 0; x < b.tileHeight; x++ {
		for y := 0; y < b.tileWidth; y++ {
			idx := x*b.tileWidth + y
			m.meshConn.AddTile([3]int{x, y, 0}, m.tilesPorts[idx])
			// dump the affiliated ports of each tile
			// fmt.Printf("Tile [%d][%d]\n\n", x, y)
			// for _, p := range m.tilesPorts[idx] {
			// 	fmt.Println(p.Name())
			// }
			// fmt.Printf("\n\n")
		}
	}
}

func (b *meshBuilder) buildMemoryLowModules(m *mesh) {
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

func (b *meshBuilder) exportTilesPorts(m *mesh) {
	b.exportCUs(m)
	b.exportROBs(m)
	b.exportATs(m)
	b.exportCaches(m)
	b.exportSRAMs(m)
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

		/* L1sAT(Control) <-> CP */
		ctrlPort = t.l1sAT.GetPortByName("Control")
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

func (b *meshBuilder) exportSRAMs(m *mesh) {
	var top sim.Port
	for idx, t := range m.tiles {
		/* Mesh Memory(Top) <-> L1vAT(Bottom) */
		/* Mesh Memory(Top) <-> L1sCache(Bottom) */
		/* Mesh Memory(Top) <-> L1iAT(Bottom) */
		top = t.mem.GetPortByName("Top")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], top)
	}
}

func (b *meshBuilder) exportCaches(m *mesh) {
	var bottom, ctrlPort sim.Port
	for idx, t := range m.tiles {
		/* L1sCache(Bottom) <-> Mesh Memory(Top) */
		/* L1sCache(Control) <-> CP */
		bottom = t.l1sCache.GetPortByName("Bottom")
		ctrlPort = t.l1sCache.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], bottom)
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.L1SCaches = append(b.cp.L1SCaches, ctrlPort)

		/* L1iCache(Control) <-> CP */
		ctrlPort = t.l1iCache.GetPortByName("Control")
		m.tilesPorts[idx] = append(m.tilesPorts[idx], ctrlPort)
		b.cp.L1ICaches = append(b.cp.L1ICaches, ctrlPort)
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
	b.populateCUs(m)
	b.populateROBs(m)
	b.populateATs(m)
	b.populateCaches(m)
	b.populateSRAMs(m)
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

func (b *meshBuilder) populateSRAMs(m *mesh) {
	for _, t := range m.tiles {
		b.gpuPtr.MemControllers = append(b.gpuPtr.MemControllers, t.mem)
	}
}

func (b *meshBuilder) populateCaches(m *mesh) {
	for _, t := range m.tiles {
		b.gpuPtr.L1SCaches = append(b.gpuPtr.L1SCaches, t.l1sCache)
		b.gpuPtr.L1ICaches = append(b.gpuPtr.L1ICaches, t.l1iCache)
	}
}

func (b *meshBuilder) fillL1TLBLowModules(m *mesh) {
	for _, t := range m.tiles {
		t.l1vTLB.LowModule = b.l2TLB.GetPortByName("Top")
		t.l1iTLB.LowModule = b.l2TLB.GetPortByName("Top")
		t.l1sTLB.LowModule = b.l2TLB.GetPortByName("Top")
	}
}

func (b *meshBuilder) fillATsAndCachesLowModules(m *mesh) {
	for _, t := range m.tiles {
		t.l1vAT.SetLowModuleFinder(m.memLowModuleFinder)
		// t.l1sAT.SetLowModuleFinder(m.memLowModuleFinder)
		t.l1sCache.SetLowModuleFinder(m.memLowModuleFinder)
		t.l1iAT.SetLowModuleFinder(m.memLowModuleFinder)
	}
}
