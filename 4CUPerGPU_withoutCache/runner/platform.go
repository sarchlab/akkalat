package runner

import (
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/driver"
	"github.com/sarchlab/mgpusim/v3/timing/cp"
	"github.com/sarchlab/mgpusim/v3/timing/pagemigrationcontroller"
	"github.com/sarchlab/mgpusim/v3/timing/rdma"
)

// TraceableComponent is a component that can accept traces
type TraceableComponent interface {
	sim.Component
	tracing.NamedHookable
}

// A Platform is a collection of the hardware under simulation.
type Platform struct {
	Engine sim.Engine
	Driver *driver.Driver
	GPUs   []*GPU
}

// A GPU is a collection of GPU internal Components
type GPU struct {
	Domain           *sim.Domain
	CommandProcessor *cp.CommandProcessor
	RDMAEngine       *rdma.Engine
	PMC              *pagemigrationcontroller.PageMigrationController
	CUs              []TraceableComponent
	L1VTLB           TraceableComponent
	L1STLB           TraceableComponent
	L1ITLB           TraceableComponent
	L2TLBs           []TraceableComponent
	MemControllers   []TraceableComponent
}
