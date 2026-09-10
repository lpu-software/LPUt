package capture

import (
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// IsSEBRunning checks if Safe Exam Browser is currently running on macOS.
func IsSEBRunning() bool {
	procs, err := process.Processes()
	if err != nil {
		return false
	}
	
	for _, p := range procs {
		name, err := p.Name()
		if err == nil {
			if strings.Contains(strings.ToLower(name), "safeexambrowser") {
				return true
			}
		}
	}
	return false
}
