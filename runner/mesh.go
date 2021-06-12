package runner

import (
	"fmt"

	"gitlab.com/akita/akita/v2/monitoring"
	"gitlab.com/akita/akita/v2/sim"
	"gitlab.com/akita/mem/v2/cache/writeback"
	"gitlab.com/akita/mem/v2/mem"
	"gitlab.com/akita/mem/v2/vm/tlb"
	"gitlab.com/akita/mgpusim/v2/rdma"
	"gitlab.com/akita/mgpusim/v2/timing/cp"
	"gitlab.com/akita/noc/v2/networking/mesh"
	"gitlab.com/akita/util/v2/tracing"
)

type MeshComponent struct {
	// cus               []*cu.ComputeUnit
	// l1vReorderBuffers []*rob.ReorderBuffer
	// l1iReorderBuffers []*rob.ReorderBuffer
	// l1sReorderBuffers []*rob.ReorderBuffer
	// l1vCaches         []*writearound.Cache
	// l1sCaches         []*l1v.Cache
	// l1iCaches         []*l1v.Cache
	// l2Caches          []*writeback.Cache
	// l1vAddrTrans      []*addresstranslator.AddressTranslator
	// l1sAddrTrans      []*addresstranslator.AddressTranslator
	// l1iAddrTrans      []*addresstranslator.AddressTranslator
	// l1vTLBs           []*tlb.TLB
	// l1sTLBs           []*tlb.TLB
	// l1iTLBs           []*tlb.TLB

	tiles       []*Tile
	l2Caches    []*writeback.Cache
	periphPorts []sim.Port
	meshConn    *mesh.Connector
}

type Tile struct {
	name                   string
	loc                    [3]int
	sa                     *shaderArray
	localL2Caches          []*writeback.Cache
	externalPorts          []sim.Port
	l1ToLocalL2Conn        *sim.DirectConnection
	L1TLBToL2TLBConnection *sim.DirectConnection
}

type tileBuilder = shaderArrayBuilder

type MeshBuilder struct {
	gpuID   uint64
	gpuName string
	name    string
	gpuPtr  *GPU
	cp      *cp.CommandProcessor
	l2TLB   *tlb.TLB

	engine            sim.Engine
	freq              sim.Freq
	memAddrOffset     uint64
	l2CacheSize       uint64
	log2CacheLineSize uint64
	log2PageSize      uint64
	visTracer         tracing.Tracer
	memTracer         tracing.Tracer
	enableMemTracing  bool
	enableVisTracing  bool
	monitor           *monitoring.Monitor

	tileWidth                      int
	tileHeight                     int
	numCUPerShaderArray            int
	numMemoryBankPerMesh           int
	numMemoryBankPerTile           int
	log2MemoryBankInterleavingSize uint64

	rdmaEngine              *rdma.Engine
	L1CachesLowModuleFinder *mem.InterleavedLowModuleFinder
}

func makeMeshBuilder() MeshBuilder {
	b := MeshBuilder{
		gpuID: 0,
		name:  "Mesh",
		freq:  1 * sim.GHz,
	}
	return b
}

func (b MeshBuilder) withGPUID(id uint64) MeshBuilder {
	b.gpuID = id
	return b
}

func (b MeshBuilder) withGPUName(str string) MeshBuilder {
	b.gpuName = str
	return b
}

func (b MeshBuilder) withGPUPtr(p *GPU) MeshBuilder {
	b.gpuPtr = p
	return b
}

func (b MeshBuilder) withCP(cp *cp.CommandProcessor) MeshBuilder {
	b.cp = cp
	return b
}

func (b MeshBuilder) withL2TLB(tlb *tlb.TLB) MeshBuilder {
	b.l2TLB = tlb
	return b
}

func (b MeshBuilder) withEngine(e sim.Engine) MeshBuilder {
	b.engine = e
	return b
}

func (b MeshBuilder) withFreq(f sim.Freq) MeshBuilder {
	b.freq = f
	return b
}

func (b MeshBuilder) WithMemAddrOffset(offset uint64) MeshBuilder {
	b.memAddrOffset = offset
	return b
}

func (b MeshBuilder) WithL2CacheSize(size uint64) MeshBuilder {
	b.l2CacheSize = size
	return b
}

func (b MeshBuilder) withLog2CachelineSize(
	log2Size uint64,
) MeshBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

func (b MeshBuilder) withLog2PageSize(
	log2Size uint64,
) MeshBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b MeshBuilder) withVisTracer(t tracing.Tracer) MeshBuilder {
	b.enableVisTracing = true
	b.visTracer = t
	return b
}

func (b MeshBuilder) withMemTracer(t tracing.Tracer) MeshBuilder {
	b.enableMemTracing = true
	b.memTracer = t
	return b
}

func (b MeshBuilder) withMonitor(monitor *monitoring.Monitor) MeshBuilder {
	b.monitor = monitor
	return b
}

func (b MeshBuilder) withTileWidth(w int) MeshBuilder {
	b.tileWidth = w
	return b
}

func (b MeshBuilder) withTileHeight(h int) MeshBuilder {
	b.tileHeight = h
	return b
}

func (b MeshBuilder) withNumCUPerShaderArray(n int) MeshBuilder {
	b.numCUPerShaderArray = n
	return b
}

func (b MeshBuilder) withNumMemoryBank(n int) MeshBuilder {
	b.numMemoryBankPerMesh = n
	return b
}

func (b MeshBuilder) withLog2MemoryBankInterleavingSize(
	n uint64,
) MeshBuilder {
	b.log2MemoryBankInterleavingSize = n
	return b
}

func (b MeshBuilder) withRDMAEngine(r *rdma.Engine) MeshBuilder {
	b.rdmaEngine = r
	return b
}

func (b *MeshBuilder) Build(
	name string,
	periphPorts []sim.Port,
) *MeshComponent {
	b.name = name
	m := MeshComponent{}
	m.periphPorts = periphPorts

	m.meshConn = mesh.NewConnector().
		WithEngine(b.engine).
		WithFreq(b.freq)
	m.meshConn.CreateNetwork(b.name)
	// b.attachPeriphPortsToMesh(&m, periphPorts)

	b.buildTiles(&m)

	m.meshConn.EstablishNetwork()

	return &m
}

func (b *MeshBuilder) attachPeriphPortsToMesh(
	m *MeshComponent,
	ports []sim.Port,
) {
	// TODO
	m.meshConn.AddTile([3]int{b.tileWidth, 0, 0}, ports)
}

func (b *MeshBuilder) attachPeriphPortsToTile0(
	m *MeshComponent,
) {
	// TODO
	m.tiles[0].externalPorts = append(m.tiles[0].externalPorts, m.periphPorts...)
}

func (b *MeshBuilder) buildL1CachesLowModuleFinder(
	m *MeshComponent,
) {
	b.L1CachesLowModuleFinder = mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)
	b.L1CachesLowModuleFinder.ModuleForOtherAddresses = b.rdmaEngine.ToL1
	b.L1CachesLowModuleFinder.UseAddressSpaceLimitation = true
	b.L1CachesLowModuleFinder.LowAddress = b.memAddrOffset
	b.L1CachesLowModuleFinder.HighAddress = b.memAddrOffset + 4*mem.GB

	b.rdmaEngine.SetLocalModuleFinder(b.L1CachesLowModuleFinder)
}

func (b *MeshBuilder) buildTiles(m *MeshComponent) {
	/* Make shader array builder */
	saBuilder := makeShaderArrayBuilder().
		withEngine(b.engine).
		withFreq(b.freq).
		withGPUID(b.gpuID).
		withLog2CachelineSize(b.log2CacheLineSize).
		withLog2PageSize(b.log2PageSize).
		withNumCU(b.numCUPerShaderArray)

	if b.enableVisTracing {
		saBuilder = saBuilder.withVisTracer(b.visTracer)
	}

	if b.enableMemTracing {
		saBuilder = saBuilder.withMemTracer(b.memTracer)
	}

	/* Make L2 cache builder */
	l2Builder := writeback.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(16).
		WithByteSize(b.l2CacheSize / uint64(b.numMemoryBankPerMesh)).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(1)
	b.numMemoryBankPerTile = b.getNumMemoryBankPerTile()
	b.buildL1CachesLowModuleFinder(m)

	for x := 0; x < b.tileWidth; x++ {
		for y := 0; y < b.tileHeight; y++ {
			tileName := fmt.Sprintf("%s.Tile_[%02d,%02d]", b.name, x, y)
			t := Tile{
				name: tileName,
				loc:  [3]int{x, y, 0},
			}

			b.buildTile(&t, saBuilder, l2Builder)
			m.l2Caches = append(m.l2Caches, t.localL2Caches...)
			b.fillL1CachesLowModules(&t)
			b.fillL1TLBLowModules(&t)
			m.tiles = append(m.tiles, &t)
		}
	}
	b.attachPeriphPortsToTile0(m)
	for _, t := range m.tiles {
		b.setL1CachesLowModuleFinder(t)
		b.populateTileComponents(t)
		b.populateTileExternalPorts(t)
		m.meshConn.AddTile(t.loc, t.externalPorts)
	}
}

func (b *MeshBuilder) buildTile(
	t *Tile,
	saBuilder shaderArrayBuilder,
	l2Builder writeback.Builder,
) {
	sa := saBuilder.Build(t.name)
	t.sa = &sa
	t.localL2Caches = b.buildL2Caches(l2Builder, t.name)
}

func (b *MeshBuilder) populateTileExternalPorts(t *Tile) {
	b.exportCUs(t)
	b.exportATsAndROBs(t)
	b.exportL1TLBs(t)
	b.exportCaches(t)
}

func (b *MeshBuilder) exportCUs(t *Tile) {
	/* CUs(ToACE) <-> CP */
	/* CUs(ToCP) <-> CP ; ToCP is control port of CU */
	for _, cu := range t.sa.cus {
		b.cp.RegisterCU(cu)
		t.externalPorts = append(t.externalPorts, cu.ToACE)
		t.externalPorts = append(t.externalPorts, cu.ToCP)
	}
}

func (b *MeshBuilder) exportATsAndROBs(t *Tile) {
	/* L1vATs(Control) <-> CP */
	for _, l1vAT := range t.sa.l1vATs {
		ctrlPort := l1vAT.GetPortByName("Control")
		t.externalPorts = append(t.externalPorts, ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
	}

	/* L1sAT(Control) <-> CP */
	l1sATCtrlPort := t.sa.l1sAT.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1sATCtrlPort)
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, l1sATCtrlPort)

	/* L1iAT(Bottom) <-> L2 caches(Top) */
	/* L1iAT(Control) <-> CP */
	l1iATBottom := t.sa.l1iAT.GetPortByName("Bottom")
	l1iATCtrlPort := t.sa.l1iAT.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1iATBottom)
	t.externalPorts = append(t.externalPorts, l1iATCtrlPort)
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, l1iATCtrlPort)

	/* L1vROBs(Control) <-> CP */
	for _, l1vROB := range t.sa.l1vROBs {
		ctrlPort := l1vROB.GetPortByName("Control")
		t.externalPorts = append(t.externalPorts, ctrlPort)
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
	}

	/* L1sROB(Control) <-> CP */
	l1sROBCtrlPort := t.sa.l1sROB.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1sROBCtrlPort)
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, l1sROBCtrlPort)

	/* L1iROB(Control) <-> CP */
	l1iROBCtrlPort := t.sa.l1iROB.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1iROBCtrlPort)
	b.cp.AddressTranslators = append(b.cp.AddressTranslators, l1iROBCtrlPort)
}

func (b *MeshBuilder) exportL1TLBs(t *Tile) {
	/* L1vTLBs(Bottom) <-> L2TLB(Top) */
	/* L1vTLBs(Control) <-> CP */
	for _, tlb := range t.sa.l1vTLBs {
		bottom := tlb.GetPortByName("Bottom")
		ctrlPort := tlb.GetPortByName("Control")
		t.externalPorts = append(t.externalPorts, bottom)
		t.externalPorts = append(t.externalPorts, ctrlPort)
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
	}

	/* L1sTLB(Bottom) <-> L2TLB(Top) */
	/* L1sTLB(Control) <-> CP */
	l1sTLBBottom := t.sa.l1sTLB.GetPortByName("Bottom")
	l1sTLBCtrlPort := t.sa.l1sTLB.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1sTLBBottom)
	t.externalPorts = append(t.externalPorts, l1sTLBCtrlPort)
	b.cp.TLBs = append(b.cp.TLBs, l1sTLBCtrlPort)

	/* L1iTLB(Bottom) <-> L2TLB(Top) */
	/* L1iTLB(Control) <-> CP */
	l1iTLBBottom := t.sa.l1iTLB.GetPortByName("Bottom")
	l1iTLBCtrlPort := t.sa.l1iTLB.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1iTLBBottom)
	t.externalPorts = append(t.externalPorts, l1iTLBCtrlPort)
	b.cp.TLBs = append(b.cp.TLBs, l1iTLBCtrlPort)

}

func (b *MeshBuilder) exportCaches(t *Tile) {
	/* L1v Caches(Bottom) <-> L2 Caches(Top) */
	/* L1v Caches(Control) <-> CP */
	for _, l1v := range t.sa.l1vCaches {
		bottom := l1v.GetPortByName("Bottom")
		ctrlPort := l1v.GetPortByName("Control")
		t.externalPorts = append(t.externalPorts, bottom)
		t.externalPorts = append(t.externalPorts, ctrlPort)
		b.cp.L1VCaches = append(b.cp.L1VCaches, ctrlPort)
	}

	/* L1s Cache(Bottom) <-> L2 Caches(Top) */
	/* L1s Cache(Control) <-> CP */
	l1sBottom := t.sa.l1sCache.GetPortByName("Bottom")
	l1sCtrlPort := t.sa.l1sCache.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1sBottom)
	t.externalPorts = append(t.externalPorts, l1sCtrlPort)
	b.cp.L1SCaches = append(b.cp.L1SCaches, l1sCtrlPort)

	/* L1i Cache(Control) <-> CP */
	l1iCtrlPort := t.sa.l1iCache.GetPortByName("Control")
	t.externalPorts = append(t.externalPorts, l1iCtrlPort)
	b.cp.L1ICaches = append(b.cp.L1ICaches, l1iCtrlPort)

	/* L2 Caches(Top) <-> L1 caches(Bottom) */
	/* L2 Caches(Bottom) <-> DRAM(Top) */
	/* L2 Caches(Control) <-> CP */
	for _, l2 := range t.localL2Caches {
		top := l2.GetPortByName("Top")
		bottom := l2.GetPortByName("Bottom")
		ctrlPort := l2.GetPortByName("Control")
		t.externalPorts = append(t.externalPorts, top)
		t.externalPorts = append(t.externalPorts, bottom)
		t.externalPorts = append(t.externalPorts, ctrlPort)
		b.cp.L2Caches = append(b.cp.L2Caches, ctrlPort)
	}
}

func (b *MeshBuilder) populateTileComponents(t *Tile) {
	// Only populate components in shader array, since the
	// L2Caches have been populated during the build stage.
	b.populateCUs(t.sa)
	b.populateROBs(t.sa)
	b.populateL1TLBs(t.sa)
	b.populateL1VAddressTranslators(t.sa)
	b.populateL1Vs(t.sa)
	b.populateScalerMemoryHierarchy(t.sa)
	b.populateInstMemoryHierarchy(t.sa)
}

func (b *MeshBuilder) populateCUs(sa *shaderArray) {
	for _, cu := range sa.cus {
		b.gpuPtr.CUs = append(b.gpuPtr.CUs, cu)

		if b.monitor != nil {
			b.monitor.RegisterComponent(cu)
		}
	}
}

func (b *MeshBuilder) populateROBs(sa *shaderArray) {
	for _, rob := range sa.l1vROBs {
		if b.monitor != nil {
			b.monitor.RegisterComponent(rob)
		}
	}
}

func (b *MeshBuilder) populateL1TLBs(sa *shaderArray) {
	for _, tlb := range sa.l1vTLBs {
		b.gpuPtr.L1VTLBs = append(b.gpuPtr.L1VTLBs, tlb)

		if b.monitor != nil {
			b.monitor.RegisterComponent(tlb)
		}
	}
}

func (b *MeshBuilder) populateL1Vs(sa *shaderArray) {
	for _, l1v := range sa.l1vCaches {
		b.gpuPtr.L1VCaches = append(b.gpuPtr.L1VCaches, l1v)

		if b.monitor != nil {
			b.monitor.RegisterComponent(l1v)
		}
	}
}

func (b *MeshBuilder) populateL1VAddressTranslators(sa *shaderArray) {
	for _, at := range sa.l1vATs {
		if b.monitor != nil {
			b.monitor.RegisterComponent(at)
		}
	}
}

func (b *MeshBuilder) populateScalerMemoryHierarchy(sa *shaderArray) {
	b.gpuPtr.L1SCaches = append(b.gpuPtr.L1SCaches, sa.l1sCache)
	b.gpuPtr.L1STLBs = append(b.gpuPtr.L1STLBs, sa.l1sTLB)
}

func (b *MeshBuilder) populateInstMemoryHierarchy(sa *shaderArray) {
	b.gpuPtr.L1ICaches = append(b.gpuPtr.L1ICaches, sa.l1iCache)
	b.gpuPtr.L1ITLBs = append(b.gpuPtr.L1ITLBs, sa.l1iTLB)
}

func (b MeshBuilder) getNumMemoryBankPerTile() int {
	numTile := b.tileHeight * b.tileWidth

	if (b.numMemoryBankPerMesh % numTile) != 0 {
		errMsg := fmt.Sprintf(
			"Memory bank number %d is not evenly divisible"+
				"by tile number %d (tile height: %d, width: %d)!\n",
			b.numMemoryBankPerMesh, numTile, b.tileHeight, b.tileWidth)
		panic(errMsg)
	}

	return b.numMemoryBankPerMesh / numTile
}

func (b *MeshBuilder) buildL2Caches(
	l2Builder writeback.Builder,
	prefixName string,
) (l2Caches []*writeback.Cache) {
	for i := 0; i < b.numMemoryBankPerTile; i++ {
		cacheName := fmt.Sprintf("%s.L2_%d", prefixName, i)
		// fmt.Println(cacheName)
		l2 := l2Builder.Build(cacheName)
		l2Caches = append(l2Caches, l2)
		b.gpuPtr.L2Caches = append(b.gpuPtr.L2Caches, l2)

		if b.enableVisTracing {
			tracing.CollectTrace(l2, b.visTracer)
		}

		if b.enableMemTracing {
			tracing.CollectTrace(l2, b.memTracer)
		}

		if b.monitor != nil {
			b.monitor.RegisterComponent(l2)
		}
	}
	return
}

func (b *MeshBuilder) fillL1TLBLowModules(t *Tile) {
	for _, l1vTLB := range t.sa.l1vTLBs {
		l1vTLB.LowModule = b.l2TLB.GetPortByName("Top")
	}
	t.sa.l1iTLB.LowModule = b.l2TLB.GetPortByName("Top")
	t.sa.l1sTLB.LowModule = b.l2TLB.GetPortByName("Top")
}

func (b *MeshBuilder) fillL1CachesLowModules(t *Tile) {
	// We can't fill the low modules during each tile build,
	// and set the low module finder of L1 caches to this
	// pointer of unfulfilled low modules, since the parameter
	// of SetLowModuleFinder is not pointer. Thus we must set
	// after all L2 caches in mesh are built.
	for _, l2 := range t.localL2Caches {
		b.L1CachesLowModuleFinder.LowModules = append(
			b.L1CachesLowModuleFinder.LowModules,
			l2.GetPortByName("Top"),
		)
	}
}

func (b *MeshBuilder) setL1CachesLowModuleFinder(t *Tile) {
	for _, l1v := range t.sa.l1vCaches {
		l1v.SetLowModuleFinder(b.L1CachesLowModuleFinder)
	}
	t.sa.l1sCache.SetLowModuleFinder(b.L1CachesLowModuleFinder)
	t.sa.l1iAT.SetLowModuleFinder(b.L1CachesLowModuleFinder)
}
