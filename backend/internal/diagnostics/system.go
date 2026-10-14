package diagnostics

import "encoding/json"

// Explicit fields only: SystemInfo also contains command lines; never emit those.
func emitSystemEvidence(method string, raw json.RawMessage, emit func(Event)) {
	if method == "SystemInfo.getProcessInfo" {
		var info struct {
			Processes []struct {
				Type string  `json:"type"`
				PID  int     `json:"id"`
				CPU  float64 `json:"cpuTime"`
			} `json:"processInfo"`
		}
		if json.Unmarshal(raw, &info) != nil {
			emit(Event{"WARN", "diagnostics.invalid_result", map[string]any{"method": method}})
			return
		}
		rows := []map[string]any{}
		for _, p := range info.Processes {
			if len(rows) < 64 {
				rows = append(rows, map[string]any{"pid": p.PID, "type": p.Type, "cpu_time_seconds": p.CPU})
			}
		}
		emit(Event{"INFO", "browser.child_processes", map[string]any{"processes": rows, "omitted": len(info.Processes) - len(rows), "source": "CDP SystemInfo.getProcessInfo", "limitation": "absence in next sample is not an observed exit or an exit code"}})
		return
	}
	var info struct {
		GPU struct {
			Devices []struct {
				VendorID int    `json:"vendorId"`
				DeviceID int    `json:"deviceId"`
				Vendor   string `json:"vendorString"`
				Device   string `json:"deviceString"`
				Driver   string `json:"driverVersion"`
			} `json:"devices"`
			Features map[string]string `json:"featureStatus"`
		} `json:"gpu"`
	}
	if json.Unmarshal(raw, &info) != nil {
		emit(Event{"WARN", "diagnostics.invalid_result", map[string]any{"method": method}})
		return
	}
	devices := []map[string]any{}
	for _, d := range info.GPU.Devices {
		if len(devices) < 8 {
			devices = append(devices, map[string]any{"vendor_id": d.VendorID, "device_id": d.DeviceID, "vendor": d.Vendor, "device": d.Device, "driver_version": d.Driver})
		}
	}
	emit(Event{"INFO", "browser.gpu_info", map[string]any{"devices": devices, "feature_status": info.GPU.Features, "source": "CDP SystemInfo.getInfo"}})
}
