package mgpu_config

import (
	"fmt"

	"gitlab.com/akita/akita/v2/sim"
	"gitlab.com/akita/mem/v2/mem"
	"gitlab.com/akita/mem/v2/vm/mmu"
	"gitlab.com/akita/mgpusim/v2/driver"
	"gitlab.com/akita/noc/v2/networking/pcie"
)

// Build builds a platform with R9Nano GPUs.
func (b R9NanoPlatformBuilder) Build() *Platform {
	engine := b.createEngine()
	if b.monitor != nil {
		b.monitor.RegisterEngine(engine)
	}

	mmuComponent, pageTable := b.createMMU(engine)

	storage := mem.NewStorage(uint64(1+b.numGPU) * 4 * mem.GB)

	gpuDriverBuilder := driver.MakeBuilder()
	if b.useMagicMemoryCopy {
		gpuDriverBuilder = gpuDriverBuilder.WithMagicMemoryCopyMiddleware()
	}
	gpuDriver := gpuDriverBuilder.
		WithEngine(engine).
		WithPageTable(pageTable).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(storage).
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

	gpuBuilder := b.createGPUBuilder(engine, gpuDriver, mmuComponent, storage)
	pcieConnector, rootComplexID :=
		b.createConnection(engine, gpuDriver, mmuComponent)

	mmuComponent.MigrationServiceProvider = gpuDriver.GetPortByName("MMU")

	rdmaAddressTable := b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createGPUs(
		rootComplexID, pcieConnector,
		gpuBuilder, gpuDriver,
		rdmaAddressTable, pmcAddressTable)

	pcieConnector.EstablishRoute()

	return &Platform{
		Engine: engine,
		Driver: gpuDriver,
		GPUs:   b.gpus,
	}
}

func (b *R9NanoPlatformBuilder) createGPUs(
	rootComplexID int,
	pcieConnector *pcie.Connector,
	gpuBuilder R9NanoGPUBuilder,
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

func (b R9NanoPlatformBuilder) createConnection(
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

func (b *R9NanoPlatformBuilder) createGPUBuilder(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
	storage *mem.Storage,
) R9NanoGPUBuilder {
	gpuBuilder := MakeR9NanoGPUBuilder().
		WithEngine(engine).
		WithMMU(mmuComponent).
		WithNumCUPerShaderArray(4).
		WithNumShaderArray(16).
		WithNumMemoryBank(16).
		WithLog2MemoryBankInterleavingSize(7).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(storage)

	if b.monitor != nil {
		gpuBuilder = gpuBuilder.WithMonitor(b.monitor)
	}

	gpuBuilder = b.setVisTracer(gpuDriver, gpuBuilder)
	gpuBuilder = b.setMemTracer(gpuBuilder)
	gpuBuilder = b.setISADebugger(gpuBuilder)

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) createGPU(
	index int,
	gpuBuilder R9NanoGPUBuilder,
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
	gpuDriver.RegisterGPU(gpu.Domain.GetPortByName("CommandProcessor"),
		4*mem.GB)
	gpu.CommandProcessor.Driver = gpuDriver.GetPortByName("GPU")

	b.configRDMAEngine(gpu, rdmaAddressTable)
	b.configPMC(gpu, gpuDriver, pmcAddressTable)

	pcieConnector.PlugInDevice(pcieSwitchID, gpu.Domain.Ports())

	b.gpus = append(b.gpus, gpu)

	return gpu
}
