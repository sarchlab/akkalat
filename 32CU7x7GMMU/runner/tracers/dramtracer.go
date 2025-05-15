package tracers

import (
	"sync"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
)

// dramTracer can trace DRAM activities.
type DramTracer struct {
	sync.Mutex
	sim.TimeTeller

	inflightTasks map[string]tracing.Task

	ReadCount       int
	WriteCount      int
	ReadAvgLatency  sim.VTimeInSec
	WriteAvgLatency sim.VTimeInSec
	ReadSize        uint64
	WriteSize       uint64
}

func NewDramTracer(timeTeller sim.TimeTeller) *DramTracer {
	return &DramTracer{
		TimeTeller:    timeTeller,
		inflightTasks: make(map[string]tracing.Task),
	}
}

// StartTask records the task start time
func (t *DramTracer) StartTask(task tracing.Task) {
	t.Lock()
	defer t.Unlock()

	task.StartTime = t.TimeTeller.CurrentTime()

	t.inflightTasks[task.ID] = task
}

// StepTask does nothing
func (t *DramTracer) StepTask(task tracing.Task) {
	// Do nothing
}

// AddMilestone does nothing
func (t *DramTracer) AddMilestone(milestone tracing.Milestone) {
	// Do nothing
}

// EndTask records the end of the task
func (t *DramTracer) EndTask(task tracing.Task) {
	t.Lock()
	defer t.Unlock()

	originalTask, ok := t.inflightTasks[task.ID]
	if !ok {
		return
	}

	task.EndTime = t.TimeTeller.CurrentTime()
	taskTime := task.EndTime - originalTask.StartTime

	switch originalTask.What {
	case "*mem.ReadReq":
		t.ReadAvgLatency = sim.VTimeInSec(
			(float64(t.ReadAvgLatency)*float64(t.ReadCount) +
				float64(taskTime)) / float64(t.ReadCount+1))
		t.ReadCount++
		t.ReadSize += originalTask.Detail.(*mem.ReadReq).AccessByteSize
	case "*mem.WriteReq":
		t.WriteAvgLatency = sim.VTimeInSec(
			(float64(t.WriteAvgLatency)*float64(t.WriteCount) +
				float64(taskTime)) / float64(t.WriteCount+1))
		t.WriteCount++
		t.WriteSize += uint64(len(originalTask.Detail.(*mem.WriteReq).Data))
	}

	delete(t.inflightTasks, task.ID)
}
