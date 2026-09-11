//go:build windows

package main

import "syscall"

func init() {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("SetProcessDPIAware")
	proc.Call()
}
