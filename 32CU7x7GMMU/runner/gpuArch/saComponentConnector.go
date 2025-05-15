package gpuArch

import (
	"fmt"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/sim/directconnection"
)

func (b *shaderArrayBuilder) connectVectorMem(sa *shaderArray) {
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		rob := sa.l1vROBs[i]
		at := sa.l1vATs[i]
		l1v := sa.l1vCaches[i]
		tlb := sa.l1vTLBs[i]

		cu.VectorMemModules = &mem.SinglePortMapper{
			Port: rob.GetPortByName("Top").AsRemote(),
		}
		b.connectWithDirectConnection(cu.ToVectorMem,
			rob.GetPortByName("Top"))

		atTopPort := at.GetPortByName("Top")
		rob.BottomUnit = atTopPort
		b.connectWithDirectConnection(
			rob.GetPortByName("Bottom"), atTopPort)

		tlbTopPort := tlb.GetPortByName("Top")
		at.SetTranslationProvider(tlbTopPort.AsRemote())
		b.connectWithDirectConnection(
			at.GetPortByName("Translation"), tlbTopPort)

		at.SetAddressToPortMapper(&mem.SinglePortMapper{
			Port: l1v.GetPortByName("Top").AsRemote(),
		})
		b.connectWithDirectConnection(l1v.GetPortByName("Top"),
			at.GetPortByName("Bottom"))
	}
}

func (b *shaderArrayBuilder) connectScalarMem(sa *shaderArray) {
	rob := sa.l1sROB
	at := sa.l1sAT
	tlb := sa.l1sTLB
	l1s := sa.l1sCache

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort.AsRemote())
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort)

	at.SetAddressToPortMapper(&mem.SinglePortMapper{
		Port: l1s.GetPortByName("Top").AsRemote(),
	})
	b.connectWithDirectConnection(
		l1s.GetPortByName("Top"), at.GetPortByName("Bottom"))

	conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.name)
	conn.PlugIn(rob.GetPortByName("Top"))
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		cu.ScalarMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToScalarMem)
	}
}

func (b *shaderArrayBuilder) connectInstMem(sa *shaderArray) {
	rob := sa.l1iROB
	at := sa.l1iAT
	tlb := sa.l1iTLB
	l1i := sa.l1iCache

	l1iTopPort := l1i.GetPortByName("Top")
	rob.BottomUnit = l1iTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), l1iTopPort)

	atTopPort := at.GetPortByName("Top")
	l1i.SetAddressToPortMapper(&mem.SinglePortMapper{
		Port: atTopPort.AsRemote(),
	})
	b.connectWithDirectConnection(l1i.GetPortByName("Bottom"), atTopPort)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort.AsRemote())
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort)

	robTopPort := rob.GetPortByName("Top")
	conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(b.name)
	conn.PlugIn(robTopPort)
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		cu.InstMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToInstMem)
	}
}

func (b *shaderArrayBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
) {
	name := fmt.Sprintf("%sto%s", port1.Name(), port2.Name())
	conn := directconnection.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		Build(name)
	conn.PlugIn(port1)
	conn.PlugIn(port2)
}
