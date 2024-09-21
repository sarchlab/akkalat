// Package runner defines how default benchmark samples are executed.
package runner

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"sort"

	// Enable profiling
	_ "net/http/pprof"
	"strconv"
	"strings"
	"sync"

	"github.com/sarchlab/akita/v3/monitoring"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/benchmarks"
	"github.com/sarchlab/mgpusim/v3/driver"
	"github.com/sarchlab/mgpusim/v3/timing/cu"
	"github.com/sarchlab/mgpusim/v3/timing/rdma"
	"github.com/tebeka/atexit"
)

// var timingFlag = flag.Bool("timing", false, "Run detailed timing simulation.")
// var maxInstCount = flag.Uint64("max-inst", 0,
// 	"Terminate the simulation after the given number of instructions is retired.")
// var parallelFlag = flag.Bool("parallel", false,
// 	"Run the simulation in parallel.")
// var isaDebug = flag.Bool("debug-isa", false, "Generate the ISA debugging file.")
// var visTracing = flag.Bool("trace-vis", false,
// 	"Generate trace for visualization purposes.")
// var visTraceStartTime = flag.Float64("trace-vis-start", -1,
// 	"The starting time to collect visualization traces. A negative number "+
// 		"represents starting from the beginning.")
// var visTraceEndTime = flag.Float64("trace-vis-end", -1,
// 	"The end time of collecting visualization traces. A negative number"+
// 		"means that the trace will be collected to the end of the simulation.")
// var verifyFlag = flag.Bool("verify", false, "Verify the emulation result.")
// var memTracing = flag.Bool("trace-mem", true, "Generate memory trace")
// var instCountReportFlag = flag.Bool("report-inst-count", false,
// 	"Report the number of instructions executed in each compute unit.")
// var cacheLatencyReportFlag = flag.Bool("report-cache-latency", false,
// 	"Report the average cache latency.")
// var cacheHitRateReportFlag = flag.Bool("report-cache-hit-rate", false,
// 	"Report the cache hit rate of each cache.")
// var tlbHitRateReportFlag = flag.Bool("report-tlb-hit-rate", false,
// 	"Report the TLB hit rate of each TLB.")
// var rdmaTransactionCountReportFlag = flag.Bool("report-rdma-transaction-count",
// 	false, "Report the number of transactions going through the RDMA engines.")
// var dramTransactionCountReportFlag = flag.Bool("report-dram-transaction-count",
// 	false, "Report the number of transactions accessing the DRAMs.")
// var useUnifiedMemoryFlag = flag.Bool("use-unified-memory", false,
// 	"Run benchmark with Unified Memory or not")
// var reportAll = flag.Bool("report-all", false, "Report all metrics to .csv file.")
// var filenameFlag = flag.String("metric-file-name", "metrics",
// 	"Modify the name of the output csv file.")
// var magicMemoryCopy = flag.Bool("magic-memory-copy", false,
// 	"Copy data from CPU directly to global memory")
// var switchLatencyFlag = flag.Int("switch-latency", 10,
// 	"The latency of the switch")
// var bandwidthFlag = flag.Int("bandwidth", 1,
// 	"The bandwidth of the network as a multiple of 16GB/s.")
// var maxNumHopsFlag = flag.Int("max-num-hops", -1,
// 	"The maximum number of hops in the network")
// var numMemBankFlag = flag.Int("num-memory-banks", 16,
// 	"The maximum number of hops in the network")
// var analyszerNameFlag = flag.String("analyzer-Name", "",
// 	"The name of the analyzer to use.")
// var analyszerPeriodFlag = flag.Float64("analyzer-period", 0.0,
// 	"The period to dump the analyzer results.")
// var visTracerDB = flag.String("trace-vis-db", "sqlite",
// 	"The database to store the visualization trace. Possible values are "+
// 		"sqlite, mysql, and csv.")
// var visTracerDBFileName = flag.String("trace-vis-db-file", "",
// 	"The file name of the database to store the visualization trace. "+
// 		"Extension names are not required. "+
// 		"If not specified, a random file name will be used. "+
// 		"This flag does not work with Mysql db. When MySQL is used, "+
// 		"the database name is always randomly generated.")

type verificationPreEnablingBenchmark interface {
	benchmarks.Benchmark

	EnableVerification()
}

type instCountTracer struct {
	tracer *instTracer
	cu     TraceableComponent
}

type cacheLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	cache  TraceableComponent
}

type rdmaLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	rdma   TraceableComponent
}

type tlbLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	tlb    TraceableComponent
}

type cacheHitRateTracer struct {
	tracer *tracing.StepCountTracer
	cache  TraceableComponent
}

type tlbHitRateTracer struct {
	tracer *tracing.StepCountTracer
	tlb    TraceableComponent
}

type rdmaTransactionCountTracer struct {
	outgoingTracer *tracing.AverageTimeTracer
	incomingTracer *tracing.AverageTimeTracer
	rdmaEngine     *rdma.Comp
}

type simdBusyTimeTracer struct {
	tracer *tracing.BusyTimeTracer
	simd   TraceableComponent
}
type dramTransactionCountTracer struct {
	tracer *dramTracer
	dram   TraceableComponent
}

type cuCPIStackTracer struct {
	cu     TraceableComponent
	tracer *cu.CPIStackTracer
}

// Runner is a class that helps running the benchmarks in the official samples.
type Runner struct {
	platform                *Platform
	maxInstStopper          *instTracer
	kernelTimeCounter       *tracing.BusyTimeTracer
	perGPUKernelTimeCounter []*tracing.BusyTimeTracer
	instCountTracers        []instCountTracer
	cacheLatencyTracers     []cacheLatencyTracer
	cacheHitRateTracers     []cacheHitRateTracer
	tlbLatencyTracers       []tlbLatencyTracer
	tlbHitRateTracers       []tlbHitRateTracer
	rdmaLatencyTracers      []rdmaLatencyTracer
	rdmaTransactionCounters []rdmaTransactionCountTracer
	dramTracers             []dramTransactionCountTracer
	benchmarks              []benchmarks.Benchmark
	monitor                 *monitoring.Monitor
	metricsCollector        *collector
	simdBusyTimeTracers     []simdBusyTimeTracer
	cuCPITraces             []cuCPIStackTracer

	Timing                     bool
	Verify                     bool
	Parallel                   bool
	ReportInstCount            bool
	ReportCacheLatency         bool
	ReportCacheHitRate         bool
	ReportTLBHitRate           bool
	ReportRDMALatency          bool
	ReportRDMATransactionCount bool
	ReportDRAMTransactionCount bool
	UseUnifiedMemory           bool
	ReportSIMDBusyTime         bool
	ReportCPIStack             bool
	ReportTLBLatency           bool

	GPUIDs []int
}

// // ParseFlag applies the runner flag to runner object
// //
// //nolint:gocyclo
// func (r *Runner) ParseFlag() *Runner {
// 	if *parallelFlag {
// 		r.Parallel = true
// 	}

// 	if *verifyFlag {
// 		r.Verify = true
// 	}

// 	if *timingFlag {
// 		r.Timing = true
// 	}

// 	if *useUnifiedMemoryFlag {
// 		r.UseUnifiedMemory = true
// 	}

// 	if *instCountReportFlag {
// 		r.ReportInstCount = true
// 	}

// 	if *cacheLatencyReportFlag {
// 		r.ReportCacheLatency = true
// 	}

// 	if *cacheHitRateReportFlag {
// 		r.ReportCacheHitRate = true
// 	}

// 	if *tlbHitRateReportFlag {
// 		r.ReportTLBHitRate = true
// 	}

// 	if *dramTransactionCountReportFlag {
// 		r.ReportDRAMTransactionCount = true
// 	}

// 	if *rdmaTransactionCountReportFlag {
// 		r.ReportRDMATransactionCount = true
// 	}

// 	if *reportAll {
// 		r.ReportInstCount = true
// 		r.ReportCacheLatency = true
// 		r.ReportCacheHitRate = true
// 		r.ReportTLBHitRate = true
// 		r.ReportDRAMTransactionCount = true
// 		r.ReportRDMATransactionCount = true
// 		r.ReportRDMALatency = true
// 		r.ReportTLBLatency = true
// 	}

// 	return r
// }

func (r *Runner) startProfilingServer() {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	fmt.Println("Profiling server running on:",
		listener.Addr().(*net.TCPAddr).Port)

	panic(http.Serve(listener, nil))
}

// Init initializes the platform simulate
func (r *Runner) Init() *Runner {
	go r.startProfilingServer()

	r.ParseFlag()

	log.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	if r.Timing {
		r.buildTimingPlatform()
	} else {
		r.buildEmuPlatform()
	}

	r.createUnifiedGPUs()

	r.defineMetrics()

	return r
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

func (r *Runner) buildEmuPlatform() {
	b := MakeEmuBuilder().
		WithNumGPU(r.GPUIDs[len(r.GPUIDs)-1])

	if r.Parallel {
		b = b.WithParallelEngine()
	}

	if *isaDebug {
		b = b.WithISADebugging()
	}

	if *visTracing {
		b = b.WithVisTracing()
	}

	if *memTracing {
		b = b.WithMemTracing()
	}

	if *magicMemoryCopy {
		b = b.WithMagicMemoryCopy()
	}

	r.platform = b.Build()
}

func (r *Runner) buildTimingPlatform() {
	b := MakeR9NanoBuilder().
		WithBandwidth(*bandwidthFlag).
		WithSwitchLatency(*switchLatencyFlag).
		WithMaxNumHops(*maxNumHopsFlag)

	if r.Parallel {
		b = b.WithParallelEngine()
	}

	if *isaDebug {
		b = b.WithISADebugging()
	}

	if *visTracing {
		b = b.WithPartialVisTracing(
			sim.VTimeInSec(*visTraceStartTime),
			sim.VTimeInSec(*visTraceEndTime),
		)
	}

	if *memTracing {
		b = b.WithMemTracing()
	}

	r.monitor = monitoring.NewMonitor()
	b = b.WithMonitor(r.monitor)

	b = r.setAnalyszer(b)

	if *magicMemoryCopy {
		b = b.WithMagicMemoryCopy()
	}

	r.platform = b.Build(*numMemBankFlag)

	r.monitor.StartServer()
}

func (*Runner) setAnalyszer(
	b R9NanoPlatformBuilder,
) R9NanoPlatformBuilder {
	if *analyszerPeriodFlag != 0 && *analyszerNameFlag == "" {
		panic("must specify -analyszer-name when using -analyszer-period")
	}

	if *analyszerNameFlag != "" {
		*analyszerNameFlag = fmt.Sprintf(*analyszerNameFlag)
		b = b.WithPerfAnalyzer(
			*analyszerNameFlag,
			*analyszerPeriodFlag,
		)
	}
	return b
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
			tracer := newInstTracer()
			r.instCountTracers = append(r.instCountTracers,
				instCountTracer{
					tracer: tracer,
					cu:     cu,
				})
			tracing.CollectTrace(cu.(tracing.NamedHookable), tracer)
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
					task.Detail.(sim.Msg).Meta().Src.Name(), "RDMA")
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
					task.Detail.(sim.Msg).Meta().Src.Name(), "RDMA")
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

func (r *Runner) addDRAMTracer() {
	if !r.ReportDRAMTransactionCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, dram := range gpu.MemControllers {
			t := dramTransactionCountTracer{}
			t.dram = dram.(TraceableComponent)
			t.tracer = newDramTracer(r.platform.Engine)
			// t.tracer = newDramTracer(r)

			tracing.CollectTrace(t.dram, t.tracer)

			r.dramTracers = append(r.dramTracers, t)
		}
	}
}

func (r *Runner) createUnifiedGPUs() {
	// unifiedGPUID := r.platform.Driver.CreateUnifiedGPU(nil, []int{
	// 	1, 2, 3, 4, 5, 6, 7, 8,
	// 	9, 10, 11, 12, 13, 14, 15, 16,
	// 	17, 18, 19, 20, 21, 22, 23, 24,
	// })
	gpulist := make([]int, 48)
	for i := 0; i < 48; i++ {
		gpulist[i] = i + 1
	}
	unifiedGPUID := r.platform.Driver.CreateUnifiedGPU(nil, gpulist)

	r.GPUIDs = []int{unifiedGPUID}
}

func (r *Runner) gpuIDStringToList(gpuIDsString string) []int {
	gpuIDs := make([]int, 0)
	gpuIDTokens := strings.Split(gpuIDsString, ",")

	for _, t := range gpuIDTokens {
		gpuID, err := strconv.Atoi(t)
		if err != nil {
			panic(err)
		}
		gpuIDs = append(gpuIDs, gpuID)
	}

	return gpuIDs
}

// AddBenchmark adds an benchmark that the driver runs
func (r *Runner) AddBenchmark(b benchmarks.Benchmark) {
	b.SelectGPU(r.GPUIDs)
	if r.UseUnifiedMemory {
		b.SetUnifiedMemory()
	}
	r.benchmarks = append(r.benchmarks, b)
}

// AddBenchmarkWithoutSettingGPUsToUse allows for user specified GPUs for
// the benchmark to run.
func (r *Runner) AddBenchmarkWithoutSettingGPUsToUse(b benchmarks.Benchmark) {
	if r.UseUnifiedMemory {
		b.SetUnifiedMemory()
	}
	r.benchmarks = append(r.benchmarks, b)
}

// Run runs the benchmark on the simulator
func (r *Runner) Run() {
	r.platform.Driver.Run()

	var wg sync.WaitGroup
	for _, b := range r.benchmarks {
		wg.Add(1)
		go func(b benchmarks.Benchmark, wg *sync.WaitGroup) {
			if r.Verify {
				if b, ok := b.(verificationPreEnablingBenchmark); ok {
					b.EnableVerification()
				}
			}

			b.Run()

			if r.Verify {
				b.Verify()
			}
			wg.Done()
		}(b, &wg)
	}
	wg.Wait()

	r.platform.Driver.Terminate()
	r.platform.Engine.Finished()

	//r.reportStats()

	atexit.Exit(0)
}

func (r *Runner) reportStats() {
	r.reportExecutionTime()
	r.reportInstCount()
	r.reportCPIStack()
	r.reportCacheLatency()
	r.reportRDMALatency()
	r.reportCacheHitRate()
	r.reportTLBHitRate()
	r.reportTLBLatency()
	r.reportRDMATransactionCount()
	r.reportDRAMTransactionCount()
	r.dumpMetrics()
}

func (r *Runner) reportInstCount() {
	// kernelTime := float64(r.kernelTimeCounter.BusyTime())
	for _, t := range r.instCountTracers {
		// kernelTime := float64(r.kernelTimeCounter.BusyTime())
		// float64(r.kernelTimeCounter.BusyTime())
		cuName := t.cu.Name()
		gpuID := regexp.MustCompile(`GPU\[(\d+)\]`)
		match := gpuID.FindStringSubmatch(cuName)
		num, err := strconv.Atoi(match[1])
		if err != nil {
			return
		}
		if num > 23 {
			num = num - 1
		}
		kernelTime := float64(r.perGPUKernelTimeCounter[num].BusyTime())

		cuFreq := float64(t.cu.(*cu.ComputeUnit).Freq)
		numCycle := kernelTime * cuFreq

		r.metricsCollector.Collect(
			t.cu.Name(), "cu_inst_count", float64(t.tracer.count))

		r.metricsCollector.Collect(
			t.cu.Name(), "cu_CPI", numCycle/float64(t.tracer.count))
	}
}

func (r *Runner) reportCPIStack() {
	for _, t := range r.cuCPITraces {
		cu := t.cu
		hook := t.tracer

		r.reportCPIStackEntries(hook, cu, false)
		// r.reportCPIStackEntries(hook, cu, true)
	}
}

func (r *Runner) reportCPIStackEntries(
	hook *cu.CPIStackTracer,
	cu TraceableComponent,
	simdStack bool,
) {
	cpiStack := hook.GetCPIStack()
	if simdStack {
		cpiStack = hook.GetSIMDCPIStack()
	}

	keys := make([]string, 0, len(cpiStack))
	for k := range cpiStack {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	stackTypeName := "CPIStack"
	if simdStack {
		stackTypeName = "SIMDCPIStack"
	}

	for _, name := range keys {
		value := cpiStack[name]
		r.metricsCollector.Collect(cu.Name(), stackTypeName+"."+name, value)
	}
}

func (r *Runner) reportExecutionTime() {
	if r.Timing {
		r.metricsCollector.Collect(
			r.platform.Driver.Name(),
			"kernel_time", float64(r.kernelTimeCounter.BusyTime()))
		r.metricsCollector.Collect(
			r.platform.Driver.Name(),
			"total_time", float64(r.platform.Engine.CurrentTime()))

		for i, c := range r.perGPUKernelTimeCounter {
			r.metricsCollector.Collect(
				r.platform.GPUs[i].CommandProcessor.Name(),
				"kernel_time", float64(c.BusyTime()))
		}
	}
}

func (r *Runner) reportCacheLatency() {
	for _, tracer := range r.cacheLatencyTracers {
		if tracer.tracer.AverageTime() == 0 {
			continue
		}

		r.metricsCollector.Collect(
			tracer.cache.Name(),
			"req_average_latency",
			float64(tracer.tracer.AverageTime()),
		)
	}
}

func (r *Runner) reportRDMALatency() {
	for _, tracer := range r.rdmaLatencyTracers {
		if tracer.tracer.AverageTime() == 0 {
			continue
		}

		r.metricsCollector.Collect(
			tracer.rdma.Name(),
			"req_average_latency",
			float64(tracer.tracer.AverageTime()),
		)
	}
}

func (r *Runner) reportTLBLatency() {
	for _, tracer := range r.tlbLatencyTracers {
		if tracer.tracer.AverageTime() == 0 {
			continue
		}

		r.metricsCollector.Collect(
			tracer.tlb.Name(),
			"req_average_latency",
			float64(tracer.tracer.AverageTime()),
		)
	}
}

func (r *Runner) reportCacheHitRate() {
	for _, tracer := range r.cacheHitRateTracers {
		readHit := tracer.tracer.GetStepCount("read-hit")
		readMiss := tracer.tracer.GetStepCount("read-miss")
		readMSHRHit := tracer.tracer.GetStepCount("read-mshr-miss")
		writeHit := tracer.tracer.GetStepCount("write-hit")
		writeMiss := tracer.tracer.GetStepCount("write-miss")
		writeMSHRHit := tracer.tracer.GetStepCount("write-mshr-miss")

		totalTransaction := readHit + readMiss + readMSHRHit +
			writeHit + writeMiss + writeMSHRHit

		if totalTransaction == 0 {
			continue
		}

		r.metricsCollector.Collect(
			tracer.cache.Name(), "read-hit", float64(readHit))
		r.metricsCollector.Collect(
			tracer.cache.Name(), "read-miss", float64(readMiss))
		r.metricsCollector.Collect(
			tracer.cache.Name(), "read-mshr-hit", float64(readMSHRHit))
		r.metricsCollector.Collect(
			tracer.cache.Name(), "write-hit", float64(writeHit))
		r.metricsCollector.Collect(
			tracer.cache.Name(), "write-miss", float64(writeMiss))
		r.metricsCollector.Collect(
			tracer.cache.Name(), "write-mshr-hit", float64(writeMSHRHit))
	}
}

func (r *Runner) reportTLBHitRate() {
	for _, tracer := range r.tlbHitRateTracers {
		hit := tracer.tracer.GetStepCount("hit")
		miss := tracer.tracer.GetStepCount("miss")
		mshrHit := tracer.tracer.GetStepCount("mshr-hit")

		totalTransaction := hit + miss + mshrHit

		if totalTransaction == 0 {
			continue
		}

		r.metricsCollector.Collect(
			tracer.tlb.Name(), "hit", float64(hit))
		r.metricsCollector.Collect(
			tracer.tlb.Name(), "miss", float64(miss))
		r.metricsCollector.Collect(
			tracer.tlb.Name(), "mshr-hit", float64(mshrHit))
	}
}

func (r *Runner) reportRDMATransactionCount() {
	for _, t := range r.rdmaTransactionCounters {
		r.metricsCollector.Collect(
			t.rdmaEngine.Name(),
			"outgoing_trans_count",
			float64(t.outgoingTracer.TotalCount()),
		)
		r.metricsCollector.Collect(
			t.rdmaEngine.Name(),
			"incoming_trans_count",
			float64(t.incomingTracer.TotalCount()),
		)
	}
}

func (r *Runner) reportDRAMTransactionCount() {
	for _, t := range r.dramTracers {
		r.metricsCollector.Collect(
			t.dram.Name(),
			"read_trans_count",
			float64(t.tracer.readCount),
		)
		r.metricsCollector.Collect(
			t.dram.Name(),
			"write_trans_count",
			float64(t.tracer.writeCount),
		)
		r.metricsCollector.Collect(
			t.dram.Name(),
			"read_avg_latency",
			float64(t.tracer.readAvgLatency),
		)
		r.metricsCollector.Collect(
			t.dram.Name(),
			"write_avg_latency",
			float64(t.tracer.writeAvgLatency),
		)
		r.metricsCollector.Collect(
			t.dram.Name(),
			"read_size",
			float64(t.tracer.readSize),
		)
		r.metricsCollector.Collect(
			t.dram.Name(),
			"write_size",
			float64(t.tracer.writeSize),
		)
	}
}

func (r *Runner) dumpMetrics() {
	r.metricsCollector.Dump(*filenameFlag)
}

// Driver returns the GPU driver used by the current runner.
func (r *Runner) Driver() *driver.Driver {
	return r.platform.Driver
}

// Engine returns the event-driven simulation engine used by the current runner.
func (r *Runner) Engine() sim.Engine {
	return r.platform.Engine
}

func (r *Runner) addMaxInstStopper() {
	if *maxInstCount == 0 {
		return
	}

	r.maxInstStopper = newInstStopper(*maxInstCount)
	for _, gpu := range r.platform.GPUs {
		for _, cu := range gpu.CUs {
			tracing.CollectTrace(cu.(tracing.NamedHookable), r.maxInstStopper)
		}
	}
}
