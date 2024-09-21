package runner

import "flag"

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
var memTracing = flag.Bool("trace-mem", true, "Generate memory trace")
var instCountReportFlag = flag.Bool("report-inst-count", false,
	"Report the number of instructions executed in each compute unit.")
var cacheLatencyReportFlag = flag.Bool("report-cache-latency", false,
	"Report the average cache latency.")
var cacheHitRateReportFlag = flag.Bool("report-cache-hit-rate", false,
	"Report the cache hit rate of each cache.")
var tlbHitRateReportFlag = flag.Bool("report-tlb-hit-rate", false,
	"Report the TLB hit rate of each TLB.")
var rdmaTransactionCountReportFlag = flag.Bool("report-rdma-transaction-count",
	false, "Report the number of transactions going through the RDMA engines.")
var dramTransactionCountReportFlag = flag.Bool("report-dram-transaction-count",
	false, "Report the number of transactions accessing the DRAMs.")
var useUnifiedMemoryFlag = flag.Bool("use-unified-memory", false,
	"Run benchmark with Unified Memory or not")
var reportAll = flag.Bool("report-all", false, "Report all metrics to .csv file.")
var filenameFlag = flag.String("metric-file-name", "metrics",
	"Modify the name of the output csv file.")
var magicMemoryCopy = flag.Bool("magic-memory-copy", false,
	"Copy data from CPU directly to global memory")
var switchLatencyFlag = flag.Int("switch-latency", 10,
	"The latency of the switch")
var bandwidthFlag = flag.Int("bandwidth", 1,
	"The bandwidth of the network as a multiple of 16GB/s.")
var maxNumHopsFlag = flag.Int("max-num-hops", -1,
	"The maximum number of hops in the network")
var numMemBankFlag = flag.Int("num-memory-banks", 16,
	"The maximum number of hops in the network")
var analyszerNameFlag = flag.String("analyzer-Name", "",
	"The name of the analyzer to use.")
var analyszerPeriodFlag = flag.Float64("analyzer-period", 0.0,
	"The period to dump the analyzer results.")
var visTracerDB = flag.String("trace-vis-db", "sqlite",
	"The database to store the visualization trace. Possible values are "+
		"sqlite, mysql, and csv.")
var visTracerDBFileName = flag.String("trace-vis-db-file", "",
	"The file name of the database to store the visualization trace. "+
		"Extension names are not required. "+
		"If not specified, a random file name will be used. "+
		"This flag does not work with Mysql db. When MySQL is used, "+
		"the database name is always randomly generated.")

// ParseFlag applies the runner flag to runner object
//
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

	if *tlbHitRateReportFlag {
		r.ReportTLBHitRate = true
	}

	if *dramTransactionCountReportFlag {
		r.ReportDRAMTransactionCount = true
	}

	if *rdmaTransactionCountReportFlag {
		r.ReportRDMATransactionCount = true
	}

	if *reportAll {
		r.ReportInstCount = true
		r.ReportCacheLatency = true
		r.ReportCacheHitRate = true
		r.ReportTLBHitRate = true
		r.ReportDRAMTransactionCount = true
		r.ReportRDMATransactionCount = true
		r.ReportRDMALatency = true
		r.ReportTLBLatency = true
		r.ReportGMMULatency = true
		r.ReportMMULatency = true
		r.ReportGMMUTransactionCount = true
		r.ReportMMUTransactionCount = true
		r.ReportSIMDBusyTime = true
		r.ReportGMMUCacheHitRate = true
		r.ReportGMMUCacheLatency = true
	}

	return r
}
