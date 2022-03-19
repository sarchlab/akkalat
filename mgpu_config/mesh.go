package mgpu_config

import (
	"fmt"

	"gitlab.com/akita/akita/v2/sim"
	"gitlab.com/akita/mem/v2/mem"
	"gitlab.com/akita/mem/v2/vm/mmu"
	"gitlab.com/akita/mgpusim/v2/driver"
	"gitlab.com/akita/noc/v2/networking/mesh"
)

// meshBuild builds a platform with R9Nano GPUs in a mesh
func (b R9NanoPlatformBuilder) meshBuild() *Platform {
	engine := b.createEngine()
	if b.monitor != nil {
		b.monitor.RegisterEngine(engine)
	}

	mmuComponent, pageTable := b.createMMU(engine)

	gpuDriver := driver.NewDriver(engine, pageTable, b.log2PageSize)
	// file, err := os.Create("driver_comm.csv")
	// if err != nil {
	// 	panic(err)
	// }
	// gpuDriver.GetPortByName("GPU").AcceptHook(
	// 	sim.NewPortMsgLogger(log.New(file, "", 0)))

	if b.monitor != nil {
		b.monitor.RegisterComponent(gpuDriver)
	}

	gpuBuilder := b.createMeshGPUBuilder(engine, gpuDriver, mmuComponent)

	meshConnector := b.createMeshConnection(engine, gpuDriver, mmuComponent)

	mmuComponent.MigrationServiceProvider = gpuDriver.GetPortByName("MMU")

	rdmaAddressTable := b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createMeshGPUs(meshConnector, gpuBuilder, gpuDriver, rdmaAddressTable,
		pmcAddressTable)

	meshConnector.EstablishNetwork()

	return &Platform{
		Engine: engine,
		Driver: gpuDriver,
		GPUs:   b.gpus,
	}
}

func (b *R9NanoPlatformBuilder) createMeshGPUs(
	meshConnector *mesh.Connector,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
) {
	for z := 0; z < b.meshSize[2]; z++ {
		for y := 0; y < b.meshSize[1]; y++ {
			for x := 0; x < b.meshSize[0]; x++ {

				// Reserving (0,0,0) for the CPU
				if x != 0 || y != 0 || z != 0 {

					i := x + (y * b.meshSize[0]) + (z * b.meshSize[0] * b.meshSize[1])

					element := [3]int{x, y, z}
					b.createMeshGPU(i, gpuBuilder, gpuDriver,
						rdmaAddressTable, pmcAddressTable,
						meshConnector, element)

					//fmt.Println("createMeshGPUs: GPU ", i, element, " GPUs", len(b.gpus))
					//meshConnector.AddTile([3]int{x, y, z}, b.gpus[i].Domain.Ports())
					//fmt.Println("createMeshGPUs: GPU ", i, " Ports", b.gpus[i-1].Domain.Ports())
				}
			}
		}
	}
}

func (b R9NanoPlatformBuilder) createMeshConnection(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMUImpl,
) *mesh.Connector {
	//connection := sim.NewDirectConnection(engine)
	// connection := noc.NewFixedBandwidthConnection(32, engine, 1*sim.GHz)
	// connection.SrcBufferCapacity = 40960000
	meshConnector := mesh.NewConnector().
		WithEngine(engine).
		WithSwitchLatency(10) // Set to 10 ns
	meshConnector.CreateNetwork("Mesh")

	meshConnector.AddTile([3]int{0, 0, 0},
		[]sim.Port{
			gpuDriver.GetPortByName("GPU"),
			gpuDriver.GetPortByName("MMU"),
			mmuComponent.GetPortByName("Migration"),
			mmuComponent.GetPortByName("Top"),
		})
	return meshConnector
}

func (b *R9NanoPlatformBuilder) createMeshGPUBuilder(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMUImpl,
) R9NanoGPUBuilder {
	gpuBuilder := MakeR9NanoGPUBuilder().
		WithEngine(engine).
		WithMMU(mmuComponent).
		WithNumCUPerShaderArray(4).
		WithNumShaderArray(4).
		WithNumMemoryBank(1).
		WithLog2MemoryBankInterleavingSize(7).
		WithLog2PageSize(b.log2PageSize)

	if b.monitor != nil {
		gpuBuilder = gpuBuilder.WithMonitor(b.monitor)
	}

	gpuBuilder = b.setVisTracer(gpuDriver, gpuBuilder)
	gpuBuilder = b.setMemTracer(gpuBuilder)
	gpuBuilder = b.setISADebugger(gpuBuilder)

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) createMeshGPU(
	index int,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
	meshConnector *mesh.Connector,
	element [3]int,
) *GPU {
	name := fmt.Sprintf("GPU%d", index)
	memAddrOffset := uint64(index) * 4 * mem.GB
	gpu := gpuBuilder.
		WithMemAddrOffset(memAddrOffset).
		Build(name, uint64(index))

	//fmt.Println("createMeshGPU: Mem Addr Offset ", index, memAddrOffset)
	gpuDriver.RegisterGPU(gpu.Domain.GetPortByName("CommandProcessor"),
		4*mem.GB)
	gpu.CommandProcessor.Driver = gpuDriver.GetPortByName("GPU")

	b.configRDMAEngine(gpu, rdmaAddressTable)
	b.configPMC(gpu, gpuDriver, pmcAddressTable)

	meshConnector.AddTile(element, gpu.Domain.Ports())

	b.gpus = append(b.gpus, gpu)

	return gpu
}
