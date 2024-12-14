package gpuArch

import (
	"fmt"

	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/sim"
)

func (b *shaderArrayBuilder) connectVectorMem(sa *shaderArray) {
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		rob := sa.l1vROBs[i]
		at := sa.l1vATs[i]
		l1v := sa.l1vCaches[i]
		tlb := sa.l1vTLBs[i]

		cu.VectorMemModules = &mem.SingleLowModuleFinder{
			LowModule: rob.GetPortByName("Top"),
		}
		b.connectWithDirectConnection(cu.ToVectorMem,
			rob.GetPortByName("Top"), 8)

		atTopPort := at.GetPortByName("Top")
		rob.BottomUnit = atTopPort
		b.connectWithDirectConnection(
			rob.GetPortByName("Bottom"), atTopPort, 8)

		tlbTopPort := tlb.GetPortByName("Top")
		at.SetTranslationProvider(tlbTopPort)
		b.connectWithDirectConnection(
			at.GetPortByName("Translation"), tlbTopPort, 8)

		at.SetLowModuleFinder(&mem.SingleLowModuleFinder{
			LowModule: l1v.GetPortByName("Top"),
		})
		b.connectWithDirectConnection(l1v.GetPortByName("Top"),
			at.GetPortByName("Bottom"), 8)
	}
}

func (b *shaderArrayBuilder) connectScalarMem(sa *shaderArray) {
	rob := sa.l1sROB
	at := sa.l1sAT
	tlb := sa.l1sTLB
	l1s := sa.l1sCache

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	at.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: l1s.GetPortByName("Top"),
	})
	b.connectWithDirectConnection(
		l1s.GetPortByName("Top"), at.GetPortByName("Bottom"), 8)

	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(rob.GetPortByName("Top"), 8)
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		cu.ScalarMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToScalarMem, 8)
	}
}

func (b *shaderArrayBuilder) connectInstMem(sa *shaderArray) {
	rob := sa.l1iROB
	at := sa.l1iAT
	tlb := sa.l1iTLB
	l1i := sa.l1iCache

	l1iTopPort := l1i.GetPortByName("Top")
	rob.BottomUnit = l1iTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), l1iTopPort, 8)

	atTopPort := at.GetPortByName("Top")
	l1i.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: atTopPort,
	})
	b.connectWithDirectConnection(l1i.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	robTopPort := rob.GetPortByName("Top")
	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(robTopPort, 8)
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		cu.InstMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToInstMem, 8)
	}
}

func (b *shaderArrayBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	name := fmt.Sprintf("%sto%s", port1.Name(), port2.Name())
	conn := sim.NewDirectConnection(
		name,
		b.engine, b.freq,
	)
	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
}
