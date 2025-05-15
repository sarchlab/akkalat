package tracers

import (
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/tebeka/atexit"
)

// instTracer can trace the number of instruction completed.
type InstTracer struct {
	Count     uint64
	SimdInst  bool
	SimdCount uint64
	MaxCount  uint64

	inflightInst map[string]tracing.Task
}

// newInstTracer creates a tracer that can count the number of instructions.
func NewInstTracer() *InstTracer {
	t := &InstTracer{
		inflightInst: map[string]tracing.Task{},
	}
	return t
}

// newInstStopper with stop the execution after a given number of instructions
// is retired.
func NewInstStopper(maxInst uint64) *InstTracer {
	t := &InstTracer{
		MaxCount:     maxInst,
		inflightInst: map[string]tracing.Task{},
	}
	return t
}

func (t *InstTracer) StartTask(task tracing.Task) {
	if task.Kind != "inst" {
		return
	}

	if task.What == "VALU" {
		t.SimdInst = true
	} else {
		t.SimdInst = false
	}

	t.inflightInst[task.ID] = task
}

func (t *InstTracer) StepTask(task tracing.Task) {
	// Do nothing
}

func (t *InstTracer) AddMilestone(milestone tracing.Milestone) {
	// Do nothing
}

func (t *InstTracer) EndTask(task tracing.Task) {
	_, found := t.inflightInst[task.ID]
	if !found {
		return
	}

	if t.SimdInst {
		t.SimdCount++
	}

	delete(t.inflightInst, task.ID)

	t.Count++

	if t.MaxCount > 0 && t.Count >= t.MaxCount {
		atexit.Exit(0)
	}
}
