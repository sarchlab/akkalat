package runner

import (
	"strings"

	"github.com/sarchlab/akita/v4/mem/vm/gmmu"
	"github.com/sarchlab/akita/v4/mem/vm/mmu"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/akkalat/32CU7x7GMMU/runner/gpuArch"
	"github.com/sarchlab/akkalat/32CU7x7GMMU/runner/tracers"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cu"
	"github.com/sarchlab/mgpusim/v4/amd/timing/rdma"
	"github.com/tebeka/atexit"
)

type instCountTracer struct {
	tracer *tracers.InstTracer
	cu     gpuArch.TraceableComponent
}

type cacheLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	cache  gpuArch.TraceableComponent
}

type rdmaLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	rdma   gpuArch.TraceableComponent
}

type mmuLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	mmu    gpuArch.TraceableComponent
}

type gmmuLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	gmmu   gpuArch.TraceableComponent
}

type tlbLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	tlb    gpuArch.TraceableComponent
}

type cacheHitRateTracer struct {
	tracer *tracing.StepCountTracer
	cache  gpuArch.TraceableComponent
}

type tlbHitRateTracer struct {
	tracer *tracing.StepCountTracer
	tlb    gpuArch.TraceableComponent
}

type rdmaTransactionCountTracer struct {
	outgoingTracer *tracing.AverageTimeTracer
	incomingTracer *tracing.AverageTimeTracer
	rdmaEngine     *rdma.Comp
}

type gmmuTransactionCountTracer struct {
	outgoingTracer *tracing.AverageTimeTracer
	incomingTracer *tracing.AverageTimeTracer
	gmmuEngine     *gmmu.Comp
}

//	type gmmuTransactionCountTracer struct {
//		tracer *mmuTracer
//		gmmu   *gmmu.GMMU
//	}
type mmuTransactionCountTracer struct {
	outgoingTracer *tracing.AverageTimeTracer
	incomingTracer *tracing.AverageTimeTracer
	mmuEngine      *mmu.Comp
}

type gmmuCacheHitRateTracer struct {
	tracer    *tracing.StepCountTracer
	gmmuCache gpuArch.TraceableComponent
}

type gmmuCacheLatencyTracer struct {
	tracer    *tracing.AverageTimeTracer
	gmmuCache gpuArch.TraceableComponent
}

type simdBusyTimeTracer struct {
	tracer *tracing.BusyTimeTracer
	simd   gpuArch.TraceableComponent
}
type dramTransactionCountTracer struct {
	tracer *tracers.DramTracer
	dram   gpuArch.TraceableComponent
}

type cuCPIStackTracer struct {
	tracer *cu.CPIStackTracer
	cu     gpuArch.TraceableComponent
}

func (r *Runner) defineMetrics() {
	r.metricsCollector = &collector{}
	r.addMaxInstStopper()
	r.addKernelTimeTracer()
	r.addInstCountTracer()
	r.addCUCPIHook()
	r.addCacheLatencyTracer()
	r.addCacheHitRateTracer()
	r.addTLBHitRateTracer()
	r.addTLBLatencyTracer()
	r.addRDMAEngineTracer()
	r.addDRAMTracer()
	r.addRDMALatencyTracer()
	r.addGMMUCacheLatencyTracer()
	r.addGMMUCacheHitRateTracer()
	r.addMMUEngineTracer()
	r.addGMMUEngineTracer()
	r.addMMULatencyTracer()
	r.addGMMULatencyTracer()
	// r.addSIMDBusyTimeTracer()

	atexit.Register(func() { r.reportStats() })
}

// func (r *Runner) addSIMDBusyTimeTracer() {
// 	if !r.ReportSIMDBusyTime {
// 		return
// 	}

// 	for _, gpu := range r.platform.GPUs {
// 		for _, simd := range gpu.SIMDs {
// 			perSIMDBusyTimeTracer := tracing.NewBusyTimeTracer(
// 				r.platform.Engine,
// 				func(task tracing.Task) bool {
// 					return task.Kind == "pipeline"
// 				})
// 			r.simdBusyTimeTracers = append(r.simdBusyTimeTracers,
// 				simdBusyTimeTracer{
// 					tracer: perSIMDBusyTimeTracer,
// 					simd:   simd,
// 				})
// 			tracing.CollectTrace(simd, perSIMDBusyTimeTracer)
// 		}
// 	}
// }

func (r *Runner) addMaxInstStopper() {
	if *maxInstCount == 0 {
		return
	}

	r.maxInstStopper = tracers.NewInstStopper(*maxInstCount)
	for _, gpu := range r.platform.GPUs {
		for _, cu := range gpu.CUs {
			tracing.CollectTrace(cu.(tracing.NamedHookable), r.maxInstStopper)
		}
	}
}

func (r *Runner) addKernelTimeTracer() {
	r.kernelTimeCounter = tracing.NewBusyTimeTracer(
		r.platform.Engine,
		func(task tracing.Task) bool {
			return task.What == "*driver.LaunchKernelCommand"
		})
	tracing.CollectTrace(r.platform.Driver, r.kernelTimeCounter)

	for _, gpu := range r.platform.GPUs {
		gpuKernelTimeCounter := tracing.NewBusyTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				return task.What == "*protocol.LaunchKernelReq"
			})
		r.perGPUKernelTimeCounter = append(
			r.perGPUKernelTimeCounter, gpuKernelTimeCounter)
		tracing.CollectTrace(gpu.CommandProcessor, gpuKernelTimeCounter)
	}
}

func (r *Runner) addInstCountTracer() {
	if !r.ReportInstCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, cu := range gpu.CUs {
			tracer := tracers.NewInstTracer()
			r.instCountTracers = append(r.instCountTracers,
				instCountTracer{
					tracer: tracer,
					cu:     cu,
				})
			tracing.CollectTrace(cu.(tracing.NamedHookable), tracer)
		}
	}
}

func (r *Runner) addCUCPIHook() {
	if !r.ReportCPIStack {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, cuComp := range gpu.CUs {
			tracer := cu.NewCPIStackInstHook(
				cuComp.(*cu.ComputeUnit), r.platform.Engine)
			tracing.CollectTrace(cuComp.(tracing.NamedHookable), tracer)

			r.cuCPITraces = append(r.cuCPITraces,
				cuCPIStackTracer{
					tracer: tracer,
					cu:     cuComp,
				})
		}
	}
}

func (r *Runner) addCacheLatencyTracer() {
	if !r.ReportCacheLatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, cache := range gpu.L1ICaches {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L1SCaches {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L1VCaches {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L2Caches {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}
	}
}

func (r *Runner) addCacheHitRateTracer() {
	if !r.ReportCacheHitRate {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, cache := range gpu.L1VCaches {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.cacheHitRateTracers = append(r.cacheHitRateTracers,
				cacheHitRateTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L1SCaches {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.cacheHitRateTracers = append(r.cacheHitRateTracers,
				cacheHitRateTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L1ICaches {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.cacheHitRateTracers = append(r.cacheHitRateTracers,
				cacheHitRateTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L2Caches {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.cacheHitRateTracers = append(r.cacheHitRateTracers,
				cacheHitRateTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}
	}
}

func (r *Runner) addRDMALatencyTracer() {
	if !r.ReportRDMALatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		rdma := gpu.RDMAEngine
		tracer := tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				return task.Kind == "req_in"
			})
		r.rdmaLatencyTracers = append(r.rdmaLatencyTracers,
			rdmaLatencyTracer{tracer: tracer, rdma: rdma})
		tracing.CollectTrace(rdma, tracer)
	}
}

func (r *Runner) addTLBLatencyTracer() {
	if !r.ReportTLBLatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, tlb := range gpu.L1VTLBs {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.tlbLatencyTracers = append(r.tlbLatencyTracers,
				tlbLatencyTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}

		for _, tlb := range gpu.L1STLBs {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.tlbLatencyTracers = append(r.tlbLatencyTracers,
				tlbLatencyTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}

		for _, tlb := range gpu.L1ITLBs {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.tlbLatencyTracers = append(r.tlbLatencyTracers,
				tlbLatencyTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}

		for _, tlb := range gpu.L2TLBs {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.tlbLatencyTracers = append(r.tlbLatencyTracers,
				tlbLatencyTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}
	}
}

func (r *Runner) addTLBHitRateTracer() {
	if !r.ReportTLBHitRate {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, tlb := range gpu.L1VTLBs {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.tlbHitRateTracers = append(r.tlbHitRateTracers,
				tlbHitRateTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}

		for _, tlb := range gpu.L1STLBs {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.tlbHitRateTracers = append(r.tlbHitRateTracers,
				tlbHitRateTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}

		for _, tlb := range gpu.L1ITLBs {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.tlbHitRateTracers = append(r.tlbHitRateTracers,
				tlbHitRateTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}

		for _, tlb := range gpu.L2TLBs {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.tlbHitRateTracers = append(r.tlbHitRateTracers,
				tlbHitRateTracer{tracer: tracer, tlb: tlb})
			tracing.CollectTrace(tlb, tracer)
		}
	}
}

func (r *Runner) addRDMAEngineTracer() {
	if !r.ReportRDMATransactionCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		t := rdmaTransactionCountTracer{}
		t.rdmaEngine = gpu.RDMAEngine
		t.incomingTracer = tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					string(task.Detail.(sim.Msg).Meta().Src), "RDMA")
				if !isFromOutside {
					return false
				}

				return true
			})
		t.outgoingTracer = tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					string(task.Detail.(sim.Msg).Meta().Src), "RDMA")
				if isFromOutside {
					return false
				}

				return true
			})

		tracing.CollectTrace(t.rdmaEngine, t.incomingTracer)
		tracing.CollectTrace(t.rdmaEngine, t.outgoingTracer)

		r.rdmaTransactionCounters = append(r.rdmaTransactionCounters, t)
	}
}

func (r *Runner) addMMUEngineTracer() {
	if !r.ReportMMUTransactionCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		t := mmuTransactionCountTracer{}
		// t.mmuEngine = gpu.MMUEngine
		t.mmuEngine = gpu.MMUEngine
		t.incomingTracer = tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					string(task.Detail.(sim.Msg).Meta().Dst), "MMU")
				if !isFromOutside {
					return false
				}

				return true
			})
		t.outgoingTracer = tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					string(task.Detail.(sim.Msg).Meta().Src), "MMU")
				if isFromOutside {
					return false
				}

				return true
			})

		tracing.CollectTrace(t.mmuEngine, t.incomingTracer)
		tracing.CollectTrace(t.mmuEngine, t.outgoingTracer)

		r.mmuTransactionCounters = append(r.mmuTransactionCounters, t)
	}
}

func (r *Runner) addGMMUEngineTracer() {
	if !r.ReportMMUTransactionCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		t := gmmuTransactionCountTracer{}
		// t := mmuTransactionCountTracer{}
		t.gmmuEngine = gpu.GMMUEngine
		t.incomingTracer = tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					string(task.Detail.(sim.Msg).Meta().Dst), "GMMU")
				if !isFromOutside {
					return false
				}

				return true
			})
		t.outgoingTracer = tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					string(task.Detail.(sim.Msg).Meta().Src), "GMMU")
				if isFromOutside {
					return false
				}

				return true
			})

		tracing.CollectTrace(t.gmmuEngine, t.incomingTracer)
		tracing.CollectTrace(t.gmmuEngine, t.outgoingTracer)

		r.gmmuTransactionCounters = append(r.gmmuTransactionCounters, t)
	}
}

func (r *Runner) addMMULatencyTracer() {
	if !r.ReportMMULatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		mmu := gpu.MMUEngine
		tracer := tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				return task.Kind == "req_in"
			})
		r.mmuLatencyTracers = append(r.mmuLatencyTracers,
			mmuLatencyTracer{tracer: tracer, mmu: mmu})
		tracing.CollectTrace(mmu, tracer)

	}
}

func (r *Runner) addGMMULatencyTracer() {
	if !r.ReportGMMULatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		gmmu := gpu.GMMUEngine
		tracer := tracing.NewAverageTimeTracer(
			r.platform.Engine,
			func(task tracing.Task) bool {
				return task.Kind == "req_in"
			})
		r.gmmuLatencyTracers = append(r.gmmuLatencyTracers,
			gmmuLatencyTracer{tracer: tracer, gmmu: gmmu})
		tracing.CollectTrace(gmmu, tracer)

	}
}

func (r *Runner) addDRAMTracer() {
	if !r.ReportDRAMTransactionCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, dram := range gpu.MemControllers {
			t := dramTransactionCountTracer{}
			t.dram = dram.(gpuArch.TraceableComponent)
			t.tracer = tracers.NewDramTracer(r.platform.Engine)
			// t.tracer = newDramTracer(r)

			tracing.CollectTrace(t.dram, t.tracer)

			r.dramTracers = append(r.dramTracers, t)
		}
	}
}

func (r *Runner) addGMMUCacheLatencyTracer() {
	if !r.ReportGMMUCacheLatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, gmmuCache := range gpu.GMMUCache {
			tracer := tracing.NewAverageTimeTracer(
				r.platform.Engine,
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.gmmuCacheLatencyTracers = append(r.gmmuCacheLatencyTracers,
				gmmuCacheLatencyTracer{tracer: tracer, gmmuCache: gmmuCache})
			tracing.CollectTrace(gmmuCache, tracer)
		}
	}
}

func (r *Runner) addGMMUCacheHitRateTracer() {
	if !r.ReportGMMUCacheHitRate {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, gmmuCache := range gpu.GMMUCache {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.gmmuCacheHitRateTracers = append(r.gmmuCacheHitRateTracers,
				gmmuCacheHitRateTracer{tracer: tracer, gmmuCache: gmmuCache})
			tracing.CollectTrace(gmmuCache, tracer)
		}
	}
}
