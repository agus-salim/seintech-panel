//go:build linux

package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	lastCPUSampleTime time.Time
	lastCPUTotal      uint64
	lastCPUIdle       uint64
	cpuMu             sync.Mutex
)

func collectPlatformMetrics(metrics *SystemMetrics) {
	collectLinuxOSRelease(metrics)
	collectLinuxLoadAvg(metrics)
	collectLinuxMeminfo(metrics)
	collectLinuxCPU(metrics)
	collectLinuxDisk(metrics)
}

func collectLinuxOSRelease(metrics *SystemMetrics) {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				val := strings.TrimPrefix(line, "PRETTY_NAME=")
				val = strings.Trim(val, "\"")
				val = strings.Replace(val, "GNU/Linux ", "", 1)
				if val != "" {
					metrics.OSRelease = val
					return
				}
			}
		}
	}
}

func collectLinuxMeminfo(metrics *SystemMetrics) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer file.Close()

	var memTotal, memFree, buffers, cached, sReclaimable, shmem, swapTotal, swapFree uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, _ := strconv.ParseUint(fields[1], 10, 64)

		switch key {
		case "MemTotal":
			memTotal = val // kB
		case "MemFree":
			memFree = val // kB
		case "Buffers":
			buffers = val // kB
		case "Cached":
			cached = val // kB
		case "SReclaimable":
			sReclaimable = val // kB
		case "Shmem":
			shmem = val // kB
		case "SwapTotal":
			swapTotal = val // kB
		case "SwapFree":
			swapFree = val // kB
		}
	}

	// Exact htop memory formula:
	// usedMem = MemTotal - MemFree - (Cached + SReclaimable - Shmem) - Buffers
	cachedTotal := cached + sReclaimable
	if cachedTotal >= shmem {
		cachedTotal -= shmem
	}

	var usedKb uint64
	if memTotal > (memFree + cachedTotal + buffers) {
		usedKb = memTotal - memFree - cachedTotal - buffers
	}

	metrics.MemoryTotalMB = memTotal / 1024
	metrics.MemoryUsedMB = usedKb / 1024
	metrics.MemoryBuffersMB = buffers / 1024
	metrics.MemoryCachedMB = (cached + sReclaimable) / 1024
	if metrics.MemoryTotalMB > 0 {
		metrics.MemoryUsagePct = float64(metrics.MemoryUsedMB) / float64(metrics.MemoryTotalMB) * 100.0
	}

	metrics.SwapTotalMB = swapTotal / 1024
	if swapTotal > swapFree {
		metrics.SwapUsedMB = (swapTotal - swapFree) / 1024
	}
}

func collectLinuxLoadAvg(metrics *SystemMetrics) {
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			metrics.LoadAvg = strings.Join(fields[0:3], ", ")
		}
	}

	if uptimeData, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(uptimeData))
		if len(fields) > 0 {
			sec, _ := strconv.ParseFloat(fields[0], 64)
			metrics.UptimeHours = sec / 3600.0
			s := int64(sec)
			days := s / 86400
			hours := (s % 86400) / 3600
			mins := (s % 3600) / 60
			if days > 0 {
				metrics.UptimeFormatted = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
			} else if hours > 0 {
				metrics.UptimeFormatted = fmt.Sprintf("%dh %dm", hours, mins)
			} else {
				metrics.UptimeFormatted = fmt.Sprintf("%dm", mins)
			}
		}
	}
}

func collectLinuxCPU(metrics *SystemMetrics) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return
	}

	var user, nice, system, idle, iowait, irq, softirq, steal uint64
	user, _ = strconv.ParseUint(fields[1], 10, 64)
	nice, _ = strconv.ParseUint(fields[2], 10, 64)
	system, _ = strconv.ParseUint(fields[3], 10, 64)
	idle, _ = strconv.ParseUint(fields[4], 10, 64)
	if len(fields) > 5 {
		iowait, _ = strconv.ParseUint(fields[5], 10, 64)
	}
	if len(fields) > 6 {
		irq, _ = strconv.ParseUint(fields[6], 10, 64)
	}
	if len(fields) > 7 {
		softirq, _ = strconv.ParseUint(fields[7], 10, 64)
	}
	if len(fields) > 8 {
		steal, _ = strconv.ParseUint(fields[8], 10, 64)
	}

	total := user + nice + system + idle + iowait + irq + softirq + steal
	idleAll := idle + iowait

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if lastCPUSampleTime.IsZero() || total <= lastCPUTotal {
		lastCPUTotal = total
		lastCPUIdle = idleAll
		lastCPUSampleTime = time.Now()

		if metrics.NumCPU > 0 && metrics.LoadAvg != "" {
			parts := strings.Split(metrics.LoadAvg, ",")
			if len(parts) > 0 {
				if l1, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
					pct := (l1 / float64(metrics.NumCPU)) * 100.0
					if pct > 100.0 {
						pct = 100.0
					}
					metrics.CPUUsagePct = pct
				}
			}
		}
		return
	}

	diffTotal := total - lastCPUTotal
	diffIdle := idleAll - lastCPUIdle
	lastCPUTotal = total
	lastCPUIdle = idleAll
	lastCPUSampleTime = time.Now()

	if diffTotal > 0 && diffTotal >= diffIdle {
		diffBusy := diffTotal - diffIdle
		metrics.CPUUsagePct = (float64(diffBusy) / float64(diffTotal)) * 100.0
	}
}

func collectLinuxDisk(metrics *SystemMetrics) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		if total > 0 {
			used := total - free
			metrics.DiskTotalGB = float64(total) / (1024 * 1024 * 1024)
			metrics.DiskUsedGB = float64(used) / (1024 * 1024 * 1024)
			metrics.DiskUsagePct = (float64(used) / float64(total)) * 100.0
		}
	}
}
