//go:build windows

package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func collectCrashEvidence(at time.Time, snapshot map[string]interface{}, core string) map[string]interface{} {
	out := map[string]interface{}{"state": "unavailable", "events": []interface{}{}, "dump_metadata": []interface{}{}, "binary_dump_exported": false}
	pid := 0
	switch value := snapshot["pid"].(type) {
	case int:
		pid = value
	case float64:
		pid = int(value)
	case uint32:
		pid = int(value)
	}
	if pid <= 0 || core == "" {
		out["reason"] = "actual PID or executable identity unavailable"
		return out
	}
	// Structured EventData only; never collect arbitrary message payloads/URLs.
	script := `param([string]$At,[int]$BrowserPID,[string]$Binary)
$ErrorActionPreference='Stop'
$when=[datetimeoffset]::Parse($At).LocalDateTime
$rows=@()
try{
 $events=Get-WinEvent -FilterHashtable @{LogName='Application';Id=1000,1001;StartTime=$when.AddMinutes(-1);EndTime=$when.AddMinutes(1)} -ErrorAction Stop
 foreach($e in $events){
  [xml]$xml=$e.ToXml();$d=@{};foreach($v in $xml.Event.EventData.Data){$d[[string]$v.Name]=[string]$v.'#text'}
  $app=$d['AppPath'];$id=$d['ProcessId'];$match=$false
  if($id){try{if($id.StartsWith('0x')){$n=[Convert]::ToInt32($id.Substring(2),16)}else{$n=[int]$id};$match=($n -eq $BrowserPID)}catch{}}
  if($match -and $app -and $app.Equals($Binary,[StringComparison]::OrdinalIgnoreCase)){
   $rows+=[pscustomobject]@{time=$e.TimeCreated.ToString('o');provider=$e.ProviderName;event_id=$e.Id;process_id=$BrowserPID;exception_code=$d['ExceptionCode'];fault_module=$d['ModuleName'];fault_module_version=$d['ModuleVersion'];fault_offset=$d['FaultingOffset']}
  }
 }
 [pscustomobject]@{state='queried';events=$rows;reason=if($rows.Count -eq 0){'no matching event; this does not prove no crash'}else{''}}|ConvertTo-Json -Depth 5 -Compress
}catch{[pscustomobject]@{state='unavailable';events=@();reason=$_.Exception.Message}|ConvertTo-Json -Compress}
`
	data, err := runPowerShellJSONWithTimeout(5*time.Second, script, "-At", at.Format(time.RFC3339Nano), "-BrowserPID", strconv.Itoa(pid), "-Binary", core)
	if err != nil {
		out["reason"] = err.Error()
	} else {
		var evidence map[string]interface{}
		if json.Unmarshal(data, &evidence) == nil {
			out = evidence
			out["binary_dump_exported"] = false
		} else {
			out["reason"] = "Windows event query returned invalid JSON"
		}
	}
	dumps := []map[string]interface{}{}
	roots := []string{filepath.Join(os.Getenv("LOCALAPPDATA"), "CrashDumps")}
	if dir, ok := snapshot["user_data_dir"].(string); ok {
		roots = append(roots, filepath.Join(dir, "Crashpad", "reports"), filepath.Join(dir, "Crashpad", "pending"))
	}
	for _, root := range roots {
		files, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".dmp") {
				continue
			}
			if root == roots[0] && !strings.HasPrefix(strings.ToLower(file.Name()), strings.ToLower(filepath.Base(core))+"."+strconv.Itoa(pid)+".") {
				continue
			}
			info, err := file.Info()
			if err != nil || info.ModTime().Before(at.Add(-time.Minute)) || info.ModTime().After(at.Add(time.Minute)) {
				continue
			}
			if len(dumps) < 10 {
				dumps = append(dumps, map[string]interface{}{"path": filepath.Join(root, file.Name()), "size_bytes": info.Size(), "modified_at": info.ModTime().Format(time.RFC3339Nano), "match_basis": fmt.Sprintf("run directory/time or executable/PID/time")})
			}
		}
	}
	out["dump_metadata"] = dumps
	out["dump_limitation"] = "metadata only; no dump generated/configured by Ant, no memory payload exported"
	return out
}
