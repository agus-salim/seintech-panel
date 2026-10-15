package system

import (
	"os"
	"runtime"
)

type SystemMetrics struct {
	Hostname        string  `json:"hostname"`
	OS              string  `json:"os"`
	Arch            string  `json:"arch"`
	OSRelease       string  `json:"os_release"`
	NumCPU          int     `json:"num_cpu"`
	CPUUsagePct     float64 `json:"cpu_usage_pct"`
	MemoryTotalMB   uint64  `json:"memory_total_mb"`
	MemoryUsedMB    uint64  `json:"memory_used_mb"`
	MemoryBuffersMB uint64  `json:"memory_buffers_mb"`
	MemoryCachedMB  uint64  `json:"memory_cached_mb"`
	MemoryUsagePct  float64 `json:"memory_usage_pct"`
	SwapTotalMB     uint64  `json:"swap_total_mb"`
	SwapUsedMB      uint64  `json:"swap_used_mb"`
	DiskTotalGB     float64 `json:"disk_total_gb"`
	DiskUsedGB      float64 `json:"disk_used_gb"`
	DiskUsagePct    float64 `json:"disk_usage_pct"`
	LoadAvg         string  `json:"load_avg"`
	UptimeHours     float64 `json:"uptime_hours"`
	UptimeFormatted string  `json:"uptime_formatted"`
}

func CollectMetrics() SystemMetrics {
	hostname, _ := os.Hostname()

	metrics := SystemMetrics{
		Hostname:        hostname,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		OSRelease:       runtime.GOOS + "/" + runtime.GOARCH,
		NumCPU:          runtime.NumCPU(),
		CPUUsagePct:     0.0,
		MemoryTotalMB:   0,
		MemoryUsedMB:    0,
		MemoryUsagePct:  0.0,
		DiskTotalGB:     50.0,
		DiskUsedGB:      12.5,
		DiskUsagePct:    25.0,
		LoadAvg:         "0.00, 0.00, 0.00",
		UptimeHours:     0.0,
		UptimeFormatted: "0m",
	}

	collectPlatformMetrics(&metrics)
	return metrics
}
