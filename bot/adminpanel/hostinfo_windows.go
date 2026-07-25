//go:build windows

package adminpanel

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	ntdll                    = syscall.NewLazyDLL("ntdll.dll")
	advapi32                 = syscall.NewLazyDLL("advapi32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
	procRtlGetVersion        = ntdll.NewProc("RtlGetVersion")
	procRegOpenKeyExW        = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW     = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey          = advapi32.NewProc("RegCloseKey")
)

type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func hostMemory() (total, avail uint64) {
	var m memoryStatusEx
	m.dwLength = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0, 0
	}
	return m.ullTotalPhys, m.ullAvailPhys
}

type filetime struct {
	dwLowDateTime, dwHighDateTime uint32
}

func (f filetime) u64() uint64 {
	return uint64(f.dwHighDateTime)<<32 | uint64(f.dwLowDateTime)
}

// hostCPUTimes 返回系统累计空闲与总 CPU 时间（100ns 单位）。
// 注意 GetSystemTimes 的 kernel 时间包含了 idle 时间。
func hostCPUTimes() (idle, total uint64, ok bool) {
	var i, k, u filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&i)),
		uintptr(unsafe.Pointer(&k)),
		uintptr(unsafe.Pointer(&u)),
	)
	if r == 0 {
		return 0, 0, false
	}
	return i.u64(), k.u64() + u.u64(), true
}

func hostUptime() uint64 {
	r, _, _ := procGetTickCount64.Call()
	return uint64(r) / 1000
}

type osVersionInfoExW struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion      uint32
	dwMinorVersion      uint32
	dwBuildNumber       uint32
	dwPlatformId        uint32
	szCSDVersion        [128]uint16
	wServicePackMajor   uint16
	wServicePackMinor   uint16
	wSuiteMask          uint16
	wProductType        byte
	wReserved           byte
}

// rtlGetVersion 通过 ntdll 获取真实系统版本（GetVersionEx 已被废弃且会撒谎）
func rtlGetVersion() (major, minor, build uint32, ok bool) {
	var v osVersionInfoExW
	v.dwOSVersionInfoSize = uint32(unsafe.Sizeof(v))
	r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&v)))
	if r != 0 { // NTSTATUS 0 = SUCCESS
		return 0, 0, 0, false
	}
	return v.dwMajorVersion, v.dwMinorVersion, v.dwBuildNumber, true
}

func hostOSVersion() string {
	major, _, build, ok := rtlGetVersion()
	if !ok {
		return "Windows"
	}
	name := "Windows"
	if major == 10 {
		if build >= 22000 {
			name = "Windows 11"
		} else {
			name = "Windows 10"
		}
	}
	return fmt.Sprintf("%s (build %d)", name, build)
}

func hostKernel() string {
	major, minor, build, ok := rtlGetVersion()
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, build)
}

// hostCPUModel 从注册表读取 CPU 友好名称，失败时回退到环境变量
func hostCPUModel() string {
	if name, err := regQueryStringValue(`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"); err == nil && name != "" {
		return name
	}
	return os.Getenv("PROCESSOR_IDENTIFIER")
}

const (
	regHKLM    = 0x80000002
	regKeyRead = 0x20019 // KEY_READ
)

// regQueryStringValue 读取 HKLM 下的字符串注册表值（避免引入 x/sys 依赖）
func regQueryStringValue(subKey, valueName string) (string, error) {
	sub, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return "", err
	}
	name, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return "", err
	}
	var h syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(regHKLM, uintptr(unsafe.Pointer(sub)), 0, regKeyRead, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return "", syscall.Errno(r)
	}
	defer procRegCloseKey.Call(uintptr(h))

	var n uint32
	r, _, _ = procRegQueryValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(name)), 0, 0, 0, uintptr(unsafe.Pointer(&n)))
	if r != 0 || n == 0 {
		return "", syscall.Errno(r)
	}
	buf := make([]uint16, n/2+1)
	r, _, _ = procRegQueryValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(name)), 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return "", syscall.Errno(r)
	}
	return syscall.UTF16ToString(buf), nil
}
