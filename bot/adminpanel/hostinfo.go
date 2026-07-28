package adminpanel

import (
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// HostSnapshot 主机硬件配置与运行状态快照（/api/host 返回）。
type HostSnapshot struct {
	Hostname   string  `json:"hostname"`
	OS         string  `json:"os"`         // windows / linux / ...
	OSVersion  string  `json:"os_version"` // 人类可读系统版本
	Kernel     string  `json:"kernel"`
	Arch       string  `json:"arch"`
	CPUModel   string  `json:"cpu_model"`
	CPUCores   int     `json:"cpu_cores"`
	CPUPercent float64 `json:"cpu_percent"` // 0-100，-1 表示不可用
	MemTotal   uint64  `json:"mem_total"`
	MemUsed    uint64  `json:"mem_used"`
	MemPercent float64 `json:"mem_percent"`
	UptimeSec  uint64  `json:"uptime_sec"` // 主机开机时长
	GoVersion  string  `json:"go_version"`
	GoMemAlloc uint64  `json:"go_mem_alloc"` // Bot 进程堆占用
	// CPUHistory 服务端缓存的 CPU 占用率历史（新在后），供面板负载图直接使用，
	// 前端打开页面即可拿到完整曲线，不必从单个数据点开始重新积累
	CPUHistory []float64 `json:"cpu_history"`
}

// ---- 平台相关采集（hostinfo_windows.go / hostinfo_linux.go / hostinfo_other.go） ----
//
//	func hostOSVersion() string                 人类可读系统版本
//	func hostKernel() string                    内核版本
//	func hostCPUModel() string                  CPU 型号
//	func hostMemory() (total, avail uint64)     物理内存总量 / 可用量
//	func hostUptime() uint64                    主机开机秒数
//	func hostCPUTimes() (idle, total, ok)       CPU 累计空闲 / 总时间片（用于差值算占用率）

// hostStatic 静态字段只采集一次
var hostStatic struct {
	sync.Once
	osVersion, kernel, cpuModel, hostname string
}

// cpuSampler 保存上次 CPU 时间片，用两次采样差值计算占用率
var cpuSampler struct {
	sync.Mutex
	primed    bool
	idle, tot uint64
}

const (
	cpuSampleInterval = 5 * time.Second // 后台 CPU 采样间隔（与面板轮询节奏一致）
	cpuHistoryCap     = 120             // 历史缓存容量（约最近 10 分钟）
)

// cpuHistory 服务端缓存的 CPU 占用率历史环形缓冲
var cpuHistory struct {
	sync.RWMutex
	started bool
	points  []float64
}

// startCPUSampler 启动后台 CPU 采样协程（幂等）。
// 采样协程是 sampleCPUPercent 的唯一调用方：占用率靠两次采样的差值计算，
// 若请求处理也直接采样会打乱差值窗口，因此 collectHost 只读取最近一次采样值。
func startCPUSampler() {
	cpuHistory.Lock()
	if cpuHistory.started {
		cpuHistory.Unlock()
		return
	}
	cpuHistory.started = true
	cpuHistory.Unlock()
	go func() {
		for {
			if p := sampleCPUPercent(); p >= 0 {
				// 保留两位小数，减小快照 JSON 体积
				p = math.Round(p*100) / 100
				cpuHistory.Lock()
				cpuHistory.points = append(cpuHistory.points, p)
				if len(cpuHistory.points) > cpuHistoryCap {
					cpuHistory.points = cpuHistory.points[len(cpuHistory.points)-cpuHistoryCap:]
				}
				cpuHistory.Unlock()
			}
			time.Sleep(cpuSampleInterval)
		}
	}()
}

// latestCPUPercent 返回最近一次后台采样的 CPU 占用率；尚无采样时返回 -1。
func latestCPUPercent() float64 {
	cpuHistory.RLock()
	defer cpuHistory.RUnlock()
	if len(cpuHistory.points) == 0 {
		return -1
	}
	return cpuHistory.points[len(cpuHistory.points)-1]
}

// cpuHistoryPoints 返回服务端缓存的 CPU 历史快照（新在后）。
func cpuHistoryPoints() []float64 {
	cpuHistory.RLock()
	defer cpuHistory.RUnlock()
	out := make([]float64, len(cpuHistory.points))
	copy(out, cpuHistory.points)
	return out
}

// sampleCPUPercent 返回距上次采样间的 CPU 占用率（0-100），不可用时返回 -1。
// 首次调用会阻塞约 150ms 完成两次采样，保证立即有值。
func sampleCPUPercent() float64 {
	i0, t0, ok := hostCPUTimes()
	if !ok {
		return -1
	}
	cpuSampler.Lock()
	if !cpuSampler.primed {
		cpuSampler.primed = true
		cpuSampler.idle, cpuSampler.tot = i0, t0
		cpuSampler.Unlock()
		time.Sleep(150 * time.Millisecond)
		if i0, t0, ok = hostCPUTimes(); !ok {
			return -1
		}
		cpuSampler.Lock()
	}
	di := i0 - cpuSampler.idle
	dt := t0 - cpuSampler.tot
	cpuSampler.idle, cpuSampler.tot = i0, t0
	cpuSampler.Unlock()
	if dt == 0 || di > dt {
		return 0
	}
	p := float64(dt-di) / float64(dt) * 100
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// collectHost 采集一份主机快照。静态字段缓存，动态字段每次实时读取；
// CPU 占用率与历史曲线来自后台采样协程的缓存（未启动采样或暂无采样时 cpu_percent 为 -1）。
func collectHost() HostSnapshot {
	hostStatic.Do(func() {
		hostStatic.osVersion = hostOSVersion()
		hostStatic.kernel = hostKernel()
		hostStatic.cpuModel = cleanCPUModel(hostCPUModel())
		hostStatic.hostname, _ = os.Hostname()
	})
	total, avail := hostMemory()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s := HostSnapshot{
		Hostname:   hostStatic.hostname,
		OS:         runtime.GOOS,
		OSVersion:  hostStatic.osVersion,
		Kernel:     hostStatic.kernel,
		Arch:       runtime.GOARCH,
		CPUModel:   hostStatic.cpuModel,
		CPUCores:   runtime.NumCPU(),
		CPUPercent: latestCPUPercent(),
		MemTotal:   total,
		UptimeSec:  hostUptime(),
		GoVersion:  runtime.Version(),
		GoMemAlloc: ms.Alloc,
		CPUHistory: cpuHistoryPoints(),
	}
	if total > 0 && avail <= total {
		s.MemUsed = total - avail
		s.MemPercent = float64(s.MemUsed) / float64(total) * 100
	}
	return s
}

// cleanCPUModel 去掉 CPU 型号里的冗余标记，压缩连续空格
func cleanCPUModel(m string) string {
	m = strings.ReplaceAll(m, "(R)", "")
	m = strings.ReplaceAll(m, "(TM)", "")
	m = strings.ReplaceAll(m, " CPU ", " ")
	return strings.Join(strings.Fields(m), " ")
}
