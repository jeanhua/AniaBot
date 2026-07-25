//go:build linux

package adminpanel

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

func hostMemory() (total, avail uint64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			continue
		}
		v *= 1024 // kB → B
		switch strings.TrimSuffix(f[0], ":") {
		case "MemTotal":
			total = v
		case "MemAvailable":
			avail = v
		}
	}
	return total, avail
}

// hostCPUTimes 解析 /proc/stat 的 cpu 汇总行，返回累计空闲与总时间片
func hostCPUTimes() (idle, total uint64, ok bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		f := strings.Fields(line)[1:]
		var vals []uint64
		for _, s := range f {
			v, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			vals = append(vals, v)
			total += v
		}
		if len(vals) >= 5 {
			idle = vals[3] + vals[4] // idle + iowait
		} else if len(vals) >= 4 {
			idle = vals[3]
		}
		return idle, total, true
	}
	return 0, 0, false
}

func hostUptime() uint64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(data))
	if len(f) == 0 {
		return 0
	}
	sec, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	return uint64(sec)
}

func hostCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func hostKernel() string {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return ""
	}
	return cstr(u.Release[:])
}

func hostOSVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return "Linux"
}

// cstr 将 Utsname 的定长 int8 字符数组转为字符串
func cstr(b []int8) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		out = append(out, byte(c))
	}
	return string(out)
}
