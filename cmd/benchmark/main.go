package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"
)

type BenchmarkResult struct {
	Suite        string        `json:"suite"`
	Scenario     string        `json:"scenario"`
	Scale        string        `json:"scale"`
	Iterations   int           `json:"iterations"`
	P50Duration  time.Duration `json:"p50_duration"`
	P95Duration  time.Duration `json:"p95_duration"`
	P99Duration  time.Duration `json:"p99_duration"`
	MeanDuration time.Duration `json:"mean_duration"`
	Throughput   float64       `json:"throughput_ops_sec"`
	BytesPerOp   int64         `json:"bytes_per_op,omitempty"`
	AllocsPerOp  int64         `json:"allocs_per_op,omitempty"`
}

type SystemMetadata struct {
	CPUModel      string `json:"cpu_model"`
	NumCPU        int    `json:"num_cpu"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"go_version"`
	GitCommit     string `json:"git_commit"`
	BenchmarkDate string `json:"benchmark_date"`
}

func getSystemMetadata() SystemMetadata {
	gitCommit := "unknown"
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	if out, err := cmd.Output(); err == nil {
		gitCommit = strings.TrimSpace(string(out))
	}

	cpuModel := "x86_64 Processor"
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					cpuModel = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	return SystemMetadata{
		CPUModel:      cpuModel,
		NumCPU:        runtime.NumCPU(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		GoVersion:     runtime.Version(),
		GitCommit:     gitCommit,
		BenchmarkDate: time.Now().UTC().Format(time.RFC3339),
	}
}

func calculatePercentiles(durations []time.Duration) (p50, p95, p99, mean time.Duration) {
	if len(durations) == 0 {
		return 0, 0, 0, 0
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	mean = total / time.Duration(len(durations))
	p50 = durations[int(float64(len(durations)-1)*0.50)]
	p95 = durations[int(float64(len(durations)-1)*0.95)]
	p99 = durations[int(float64(len(durations)-1)*0.99)]
	return
}

func main() {
	cpuProfile := flag.String("cpuprofile", "", "write cpu profile to file")
	memProfile := flag.String("memprofile", "", "write memory profile to file")
	flag.Parse()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err == nil {
			defer f.Close()
			_ = pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}
	}

	meta := getSystemMetadata()
	fmt.Printf("\n================================================================================\n")
	fmt.Printf("           SENTINELMESH STAGE 16: COMPREHENSIVE BENCHMARK RUNNER                \n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("  CPU: %s (%d cores)\n", meta.CPUModel, meta.NumCPU)
	fmt.Printf("  Platform: %s/%s | Go: %s | Git: %s\n", meta.OS, meta.Arch, meta.GoVersion, meta.GitCommit)
	fmt.Printf("================================================================================\n\n")

	var allResults []BenchmarkResult

	// 1. Scheduler Scalability Benchmarks
	allResults = append(allResults, runSchedulerBenchmarks()...)

	// 2. Messaging Pipeline Benchmarks
	allResults = append(allResults, runMessagingBenchmarks()...)

	// 3. Checkpoint Multi-Tier Benchmarks
	allResults = append(allResults, runCheckpointBenchmarks()...)

	// 4. Concurrent Recovery Benchmarks
	allResults = append(allResults, runRecoveryBenchmarks()...)

	// 5. Verification Overhead Benchmarks
	allResults = append(allResults, runVerificationBenchmarks()...)

	// 6. Full E2E Lifecycle Benchmark
	allResults = append(allResults, runE2EBenchmarks()...)

	// Save JSON and CSV results
	saveResults(meta, allResults)

	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err == nil {
			defer f.Close()
			runtime.GC()
			_ = pprof.WriteHeapProfile(f)
		}
	}

	fmt.Println("\n================================================================================")
	fmt.Println("                      STAGE 16 BENCHMARK SUITE COMPLETE                         ")
	fmt.Println("================================================================================")
}

func saveResults(meta SystemMetadata, results []BenchmarkResult) {
	outDir := "benchmark/results"
	_ = os.MkdirAll(outDir, 0755)

	// Save JSON
	jsonPayload, _ := json.MarshalIndent(map[string]interface{}{
		"system":  meta,
		"results": results,
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "benchmark_data.json"), jsonPayload, 0644)

	// Save CSV
	csvFile, err := os.Create(filepath.Join(outDir, "benchmark_data.csv"))
	if err == nil {
		defer csvFile.Close()
		w := csv.NewWriter(csvFile)
		_ = w.Write([]string{"Suite", "Scenario", "Scale", "Iterations", "P50 (ns)", "P95 (ns)", "P99 (ns)", "Mean (ns)", "Throughput (ops/sec)"})
		for _, r := range results {
			_ = w.Write([]string{
				r.Suite, r.Scenario, r.Scale, fmt.Sprintf("%d", r.Iterations),
				fmt.Sprintf("%d", r.P50Duration.Nanoseconds()),
				fmt.Sprintf("%d", r.P95Duration.Nanoseconds()),
				fmt.Sprintf("%d", r.P99Duration.Nanoseconds()),
				fmt.Sprintf("%d", r.MeanDuration.Nanoseconds()),
				fmt.Sprintf("%.2f", r.Throughput),
			})
		}
		w.Flush()
	}
	fmt.Printf("\n[OUTPUT] Raw data persisted to %s/benchmark_data.json and .csv\n", outDir)
}
