//go:build !windows && !linux

package adminpanel

import "runtime"

// 其他平台的回退实现：仅保证可编译，动态数据返回不可用值

func hostMemory() (total, avail uint64) { return 0, 0 }

func hostCPUTimes() (idle, total uint64, ok bool) { return 0, 0, false }

func hostUptime() uint64 { return 0 }

func hostCPUModel() string { return "" }

func hostKernel() string { return "" }

func hostOSVersion() string { return runtime.GOOS }
