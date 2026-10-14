//go:build windows

package backend

import (
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"time"
	"unsafe"
)

var evidenceKernelDLL = windows.NewLazySystemDLL("kernel32.dll")
var evidenceMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
var evidenceHandleCount = evidenceKernelDLL.NewProc("GetProcessHandleCount")
var evidenceMemoryStatus = evidenceKernelDLL.NewProc("GlobalMemoryStatusEx")
var evidenceDiskSpace = evidenceKernelDLL.NewProc("GetDiskFreeSpaceExW")

// A retained handle belongs to one process generation, including after exit.
// Opening another process with a reused PID cannot replace this identity.
type processEvidenceHandle struct {
	handle  windows.Handle
	pid     int
	created int64
	openErr error
}

func openProcessEvidence(pid int) *processEvidenceHandle {
	p := &processEvidenceHandle{pid: pid}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		h, err = windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	}
	if err != nil {
		p.openErr = err
		return p
	}
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		windows.CloseHandle(h)
		p.openErr = err
		return p
	}
	p.handle = h
	p.created = created.Nanoseconds()
	return p
}

func (p *processEvidenceHandle) snapshot() map[string]interface{} {
	out := map[string]interface{}{"pid": p.pid, "created_at_ns": p.created, "state": "unknown", "exit_code": nil, "source": "windows_process_handle"}
	if p.openErr != nil {
		out["error"] = p.openErr.Error()
		return out
	}
	result, err := windows.WaitForSingleObject(p.handle, 0)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	if result == uint32(windows.WAIT_TIMEOUT) {
		out["state"] = "alive"
	} else if result == uint32(windows.WAIT_OBJECT_0) {
		out["state"] = "exited"
		var code uint32
		if err := windows.GetExitCodeProcess(p.handle, &code); err != nil {
			out["error"] = err.Error()
		} else {
			out["exit_code"] = code
			out["exit_code_hex"] = fmt.Sprintf("0x%08X", code)
		}
	} else {
		out["error"] = fmt.Sprintf("unexpected process wait result %d", result)
	}
	if out["state"] == "alive" {
		var count uint32
		okHandles, _, handleErr := evidenceHandleCount.Call(uintptr(p.handle), uintptr(unsafe.Pointer(&count)))
		if okHandles != 0 {
			out["handle_count"] = count
		} else {
			out["handle_count_error"] = handleErr.Error()
		}
		// Query limits may not permit all resource counters. Failure stays explicit.
		counters := struct {
			Size, Faults                                                                          uint32
			PeakWS, WS, PeakPaged, Paged, PeakNonPaged, NonPaged, Pagefile, PeakPagefile, Private uintptr
		}{}
		counters.Size = uint32(unsafe.Sizeof(counters))
		ok, _, err := evidenceMemoryInfo.Call(uintptr(p.handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
		if ok != 0 {
			out["working_set_bytes"] = uint64(counters.WS)
			out["private_bytes"] = uint64(counters.Private)
		} else {
			out["resource_error"] = err.Error()
		}
	}
	return out
}

func (p *processEvidenceHandle) close() {
	if p.handle != 0 {
		windows.CloseHandle(p.handle)
		p.handle = 0
	}
}

func systemResourceEvidence() map[string]interface{} {
	mem := struct {
		Length, Load                                                                          uint32
		TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, AvailExtended uint64
	}{}
	mem.Length = uint32(unsafe.Sizeof(mem))
	ok, _, err := evidenceMemoryStatus.Call(uintptr(unsafe.Pointer(&mem)))
	if ok == 0 {
		return map[string]interface{}{"state": "unavailable", "error": err.Error()}
	}
	return map[string]interface{}{"captured_at": time.Now().Format(time.RFC3339Nano), "memory_load_percent": mem.Load, "available_memory_bytes": mem.AvailPhys, "commit_limit_bytes": mem.TotalPage, "available_commit_bytes": mem.AvailPage}
}

func diagnosticDiskEvidence(path string) map[string]interface{} {
	volume := filepath.VolumeName(path)
	if volume == "" {
		return map[string]interface{}{"state": "unavailable", "reason": "diagnostics volume unavailable"}
	}
	name, err := windows.UTF16PtrFromString(volume + string(filepath.Separator))
	if err != nil {
		return map[string]interface{}{"state": "unavailable", "reason": err.Error()}
	}
	var available, total, free uint64
	ok, _, callErr := evidenceDiskSpace.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if ok == 0 {
		return map[string]interface{}{"state": "unavailable", "reason": callErr.Error()}
	}
	return map[string]interface{}{"volume": volume, "available_bytes": available, "total_bytes": total, "free_bytes": free, "state": "captured"}
}
