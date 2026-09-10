package sysinfo

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"

	"github.com/yatishydv/lput/internal/protocol"
)

// Collect gathers system information from the current machine.
func Collect() (*protocol.SystemInfoData, error) {
	hostname, _ := os.Hostname()
	osVersion := runtime.GOOS
	arch := runtime.GOARCH

	// Host info
	hostInfo, err := host.Info()
	if err == nil {
		osVersion = fmt.Sprintf("%s %s", hostInfo.Platform, hostInfo.PlatformVersion)
	}

	// CPU
	cpuInfo, _ := cpu.Info()
	cpuModel := ""
	cpuCores := runtime.NumCPU()
	if len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	}

	// Memory
	memInfo, _ := mem.VirtualMemory()
	memTotalMB := uint64(0)
	memUsedMB := uint64(0)
	if memInfo != nil {
		memTotalMB = memInfo.Total / (1024 * 1024)
		memUsedMB = memInfo.Used / (1024 * 1024)
	}

	// Uptime
	uptime, _ := host.Uptime()

	// Current user
	currentUser := ""
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}

	// Running processes (top 50 by CPU)
	procs, _ := process.Processes()
	var procInfos []protocol.ProcessInfo
	for _, p := range procs {
		if len(procInfos) >= 50 {
			break
		}
		name, _ := p.Name()
		cpuPct, _ := p.CPUPercent()
		memInfo, _ := p.MemoryInfo()
		memMB := float32(0)
		if memInfo != nil {
			memMB = float32(memInfo.RSS) / (1024 * 1024)
		}
		status, _ := p.Status()
		statusStr := ""
		if len(status) > 0 {
			statusStr = status[0]
		}

		procInfos = append(procInfos, protocol.ProcessInfo{
			PID:        p.Pid,
			Name:       name,
			CPUPercent: cpuPct,
			MemoryMB:   memMB,
			Status:     statusStr,
		})
	}

	return &protocol.SystemInfoData{
		Hostname:      hostname,
		OS:            runtime.GOOS,
		OSVersion:     osVersion,
		Arch:          arch,
		CPUModel:      cpuModel,
		CPUCores:      cpuCores,
		MemoryTotalMB: memTotalMB,
		MemoryUsedMB:  memUsedMB,
		Uptime:        time.Duration(uptime) * time.Second,
		CurrentUser:   currentUser,
		Processes:     procInfos,
	}, nil
}
