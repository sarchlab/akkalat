// Package runner defines how default benchmark samples are executed.
package mgpu_config

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	// Enable profiling
	_ "net/http/pprof"
	"strconv"
	"strings"
	"sync"

	"github.com/tebeka/atexit"
	"gitlab.com/akita/akita/v2/monitoring"
	"gitlab.com/akita/akita/v2/sim"
	"gitlab.com/akita/mgpusim/v2/benchmarks"
	"gitlab.com/akita/mgpusim/v2/driver"
	"gitlab.com/akita/mgpusim/v2/timing/rdma"
	"gitlab.com/akita/util/v2/tracing"
)

var timingFlag = flag.Bool("timing", false, "Run detailed timing simulation.")
var maxInstCount = flag.Uint64("max-inst", 0,
	"Terminate the simulation after the given number of instructions is retired.")
var parallelFlag = flag.Bool("parallel", false,
	"Run the simulation in parallel.")
var isaDebug = flag.Bool("debug-isa", false, "Generate the ISA debugging file.")
var visTracing = flag.Bool("trace-vis", false,
	"Generate trace for visualization purposes.")
var visTraceStartTime = flag.Float64("trace-vis-start", -1,
	"The starting time to collect visualization traces. A negative number "+
		"represents starting from the beginning.")
var visTraceEndTime = flag.Float64("trace-vis-end", -1,
	"The end time of collecting visualization traces. A negative number"+
		"means that the trace will be collected to the end of the simulation.")
var verifyFlag = flag.Bool("verify", false, "Verify the emulation result.")
var memTracing = flag.Bool("trace-mem", false, "Generate memory trace")
var instCountReportFlag = flag.Bool("report-inst-count", false,
	"Report the number of instructions executed in each compute unit.")
var cacheLatencyReportFlag = flag.Bool("report-cache-latency", false,
	"Report the average cache latency.")
var cacheHitRateReportFlag = flag.Bool("report-cache-hit-rate", false,
	"Report the cache hit rate of each cache.")
var rdmaTransactionCountReportFlag = flag.Bool("report-rdma-transaction-count",
	false, "Report the number of transactions going through the RDMA engines.")
var dramTransactionCountReportFlag = flag.Bool("report-dram-transaction-count",
	false, "Report the number of transactions accessing the DRAMs.")
var gpuFlag = flag.String("gpus", "",
	"The GPUs to use, use a format like 1,2,3,4. By default, GPU 1 is used.")
var meshFlag = flag.String("mesh", "",
	"The GPUs to use, use a format like width,height,depth.")
var unifiedGPUFlag = flag.String("unified-gpus", "",
	`Run multi-GPU benchmark in a unified mode.
Use a format like 1,2,3,4. Cannot coexist with -gpus.`)
var useUnifiedMemoryFlag = flag.Bool("use-unified-memory", false,
	"Run benchmark with Unified Memory or not")
var reportAll = flag.Bool("report-all", false, "Report all metrics to .csv file.")
var filenameFlag = flag.String("metric-file-name", "metrics",
	"Modify the name of the output csv file.")

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

type cacheHitRateTracer struct {
	tracer *tracing.StepCountTracer
	cache  TraceableComponent
}

type dramTransactionCountTracer struct {
	tracer *dramTracer
	dram   TraceableComponent
}

type rdmaTransactionCountTracer struct {
	outgoingTracer *tracing.AverageTimeTracer
	incomingTracer *tracing.AverageTimeTracer
	rdmaEngine     *rdma.Engine
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
	rdmaTransactionCounters []rdmaTransactionCountTracer
	dramTracers             []dramTransactionCountTracer
	benchmarks              []benchmarks.Benchmark
	monitor                 *monitoring.Monitor
	metricsCollector        *collector

	Timing                     bool
	Verify                     bool
	Parallel                   bool
	ReportInstCount            bool
	ReportCacheLatency         bool
	ReportCacheHitRate         bool
	ReportRDMATransactionCount bool
	ReportDRAMTransactionCount bool
	UseUnifiedMemory           bool
	Mesh                       bool

	meshSize [3]int
	GPUIDs   []int
}

// ParseFlag applies the runner flag to runner object
//nolint:gocyclo
func (r *Runner) ParseFlag() *Runner {
	if *parallelFlag {
		r.Parallel = true
	}

	if *verifyFlag {
		r.Verify = true
	}

	if *timingFlag {
		r.Timing = true
	}

	if *useUnifiedMemoryFlag {
		r.UseUnifiedMemory = true
	}

	if *instCountReportFlag {
		r.ReportInstCount = true
	}

	if *cacheLatencyReportFlag {
		r.ReportCacheLatency = true
	}

	if *cacheHitRateReportFlag {
		r.ReportCacheHitRate = true
	}

	if *dramTransactionCountReportFlag {
		r.ReportDRAMTransactionCount = true
	}

	if *rdmaTransactionCountReportFlag {
		r.Timing = true // added due to rdma dependency on -timing
		r.ReportRDMATransactionCount = true
	}

	if *reportAll {
		r.Timing = true // added due to rdma dependency on -timing
		r.ReportInstCount = true
		r.ReportCacheLatency = true
		r.ReportCacheHitRate = true
		r.ReportDRAMTransactionCount = true
		r.ReportRDMATransactionCount = true
	}

	return r
}

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

	if *meshFlag != "" {
		r.parseMeshFlag()
	} else {
		r.parseGPUFlag()
	}

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
	r.addCacheLatencyTracer()
	r.addCacheHitRateTracer()
	r.addRDMAEngineTracer()
	r.addDRAMTracer()

	atexit.Register(func() { r.reportStats() })
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

	r.platform = b.Build()
}

func (r *Runner) buildTimingPlatform() {
	b := MakeR9NanoBuilder().
		WithNumGPU(r.GPUIDs[len(r.GPUIDs)-1]).
		WithMesh(r.Mesh, r.meshSize)

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

	if r.Mesh {
		r.platform = b.meshBuild()
	} else {
		r.platform = b.Build()

	}

	r.monitor.StartServer()
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

func (r *Runner) addKernelTimeTracer() {
	r.kernelTimeCounter = tracing.NewBusyTimeTracer(
		func(task tracing.Task) bool {
			return task.What == "*driver.LaunchKernelCommand"
		})
	tracing.CollectTrace(r.platform.Driver, r.kernelTimeCounter)

	for _, gpu := range r.platform.GPUs {
		gpuKernelTimeCounter := tracing.NewBusyTimeTracer(
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

func (r *Runner) addCacheLatencyTracer() {
	if !r.ReportCacheLatency {
		return
	}

	for _, gpu := range r.platform.GPUs {
		for _, cache := range gpu.L1ICaches {
			tracer := tracing.NewAverageTimeTracer(
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L1SCaches {
			tracer := tracing.NewAverageTimeTracer(
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L1VCaches {
			tracer := tracing.NewAverageTimeTracer(
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				cacheLatencyTracer{tracer: tracer, cache: cache})
			tracing.CollectTrace(cache, tracer)
		}

		for _, cache := range gpu.L2Caches {
			tracer := tracing.NewAverageTimeTracer(
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

func (r *Runner) addRDMAEngineTracer() {
	if !r.ReportRDMATransactionCount {
		return
	}

	for _, gpu := range r.platform.GPUs {
		t := rdmaTransactionCountTracer{}
		t.rdmaEngine = gpu.RDMAEngine
		t.incomingTracer = tracing.NewAverageTimeTracer(
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					task.Detail.(sim.Msg).Meta().Src.Name(), "RDMA")

				return isFromOutside
			})
		t.outgoingTracer = tracing.NewAverageTimeTracer(
			func(task tracing.Task) bool {
				if task.Kind != "req_in" {
					return false
				}

				isFromOutside := strings.Contains(
					task.Detail.(sim.Msg).Meta().Src.Name(), "RDMA")

				return !isFromOutside
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
			t.tracer = newDramTracer()

			tracing.CollectTrace(t.dram, t.tracer)

			r.dramTracers = append(r.dramTracers, t)
		}
	}
}

func (r *Runner) parseGPUFlag() {
	if *gpuFlag == "" && *unifiedGPUFlag == "" {
		r.GPUIDs = []int{1}
		return
	}

	if *gpuFlag != "" && *unifiedGPUFlag != "" {
		panic("cannot use -gpus and -unified-gpus together")
	}

	var gpuIDs []int
	if *gpuFlag != "" {
		gpuIDs = r.gpuIDStringToList(*gpuFlag)
	} else if *unifiedGPUFlag != "" {
		gpuIDs = r.gpuIDStringToList(*unifiedGPUFlag)
	}

	r.GPUIDs = gpuIDs
	//fmt.Println("parseGPUFlag executed", r.GPUIDs)
}

func (r *Runner) parseMeshFlag() {
	var numGPUs int
	var err error
	index := 0
	gpuIDs := make([]int, 0)

	r.meshSize = [3]int{1, 1, 1}

	if *gpuFlag != "" || *unifiedGPUFlag != "" {
		panic("cannot use -gpus or -unified-gpus with -mesh")
	}

	meshTokens := strings.Split(*meshFlag, ",")

	for _, t := range meshTokens {
		r.meshSize[index], err = strconv.Atoi(t)
		if err != nil {
			panic(err)
		}
		index++
	}

	// Minus one to account for CPU
	numGPUs = (r.meshSize[0] * r.meshSize[1] * r.meshSize[2]) - 1

	if numGPUs > 0 {
		r.Mesh = true
	}

	for x := 1; x <= numGPUs; x++ {
		gpuIDs = append(gpuIDs, x)
	}

	r.GPUIDs = gpuIDs
	//fmt.Println("parseMeshFlag executed", r.GPUIDs)
}

func (r *Runner) createUnifiedGPUs() {
	if *unifiedGPUFlag == "" && *meshFlag == "" {
		return
	}

	unifiedGPUID := r.platform.Driver.CreateUnifiedGPU(nil, r.GPUIDs)
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
	r.reportCacheLatency()
	r.reportCacheHitRate()
	r.reportRDMATransactionCount()
	r.reportDRAMTransactionCount()
	r.dumpMetrics()
}

func (r *Runner) reportInstCount() {
	for _, t := range r.instCountTracers {
		r.metricsCollector.Collect(
			t.cu.Name(), "inst_count", float64(t.tracer.count))
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
