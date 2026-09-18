package automation

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type memorySessionProcess struct {
	mu     sync.Mutex
	writes bytes.Buffer
	stdout io.Reader
	killed bool
}

func (p *memorySessionProcess) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writes.Write(data)
}

func (p *memorySessionProcess) Stdout() io.Reader { return p.stdout }

func (p *memorySessionProcess) Kill() error {
	p.mu.Lock()
	p.killed = true
	p.mu.Unlock()
	return nil
}

func TestPageSessionAwaitReadyAndCall(t *testing.T) {
	proc := &memorySessionProcess{stdout: strings.NewReader(
		`{"type":"ready","session":{"profileId":"profile-a"}}` + "\n" +
			`{"id":1,"ok":true,"result":{"url":"https://example.com"}}` + "\n",
	)}
	session := &pageSession{
		profileID: "profile-a",
		proc:      proc,
		reader:    newBufferedSessionReader(proc.Stdout()),
		lastUsed:  time.Now(),
	}

	if err := session.awaitReady(time.Second); err != nil {
		t.Fatalf("awaitReady returned error: %v", err)
	}
	if session.session["profileId"] != "profile-a" {
		t.Fatalf("session metadata = %#v", session.session)
	}
	step, err := session.call(PageCommand{Action: "snapshot"}, time.Second)
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}
	if !step.OK || step.Action != "snapshot" || step.Result["url"] != "https://example.com" {
		t.Fatalf("step = %#v", step)
	}
	if got := proc.writes.String(); !strings.Contains(got, `"id":1`) || !strings.Contains(got, `"action":"snapshot"`) {
		t.Fatalf("command write = %q", got)
	}
}

func TestPageSessionCloseKillsProcess(t *testing.T) {
	proc := &memorySessionProcess{stdout: strings.NewReader("")}
	session := &pageSession{proc: proc, reader: newBufferedSessionReader(proc.Stdout()), lastUsed: time.Now()}
	session.close()
	if !session.isClosed() {
		t.Fatal("session should be closed")
	}
	proc.mu.Lock()
	killed := proc.killed
	proc.mu.Unlock()
	if !killed {
		t.Fatal("session close did not kill process")
	}
}

func TestRunnerAssetsIncludePageSessionFiles(t *testing.T) {
	for _, name := range []string{"runner_browser.cjs", pageSessionRunnerFileName} {
		content, ok := runnerAssetFiles[name]
		if !ok || len(content) == 0 {
			t.Fatalf("runner asset %q is missing", name)
		}
	}
}

func TestPageSessionConfigKeyIsStableForEquivalentSelectorMaps(t *testing.T) {
	state := RuntimeState{RuntimeDir: `C:\runtime`, NodePath: `C:\node.exe`}
	left := PageCommandRequest{
		ProfileID:        "profile-a",
		Selector:         map[string]any{"profileId": "profile-a", "launchCode": "ABC"},
		LaunchBaseURL:    "http://127.0.0.1:19000",
		LaunchAuthHeader: "Authorization",
		LaunchAuthValue:  "Bearer test",
		ArtifactDir:      `C:\artifacts`,
		Timeout:          15 * time.Second,
	}
	right := left
	right.Selector = map[string]any{"launchCode": "ABC", "profileId": "profile-a"}

	leftKey, err := pageSessionConfigKey(state, left)
	if err != nil {
		t.Fatalf("left config key: %v", err)
	}
	rightKey, err := pageSessionConfigKey(state, right)
	if err != nil {
		t.Fatalf("right config key: %v", err)
	}
	if leftKey != rightKey {
		t.Fatalf("equivalent selector maps produced different keys: %s != %s", leftKey, rightKey)
	}
}

func TestPageSessionConfigKeyChangesWhenSessionConfigurationChanges(t *testing.T) {
	baseState := RuntimeState{RuntimeDir: `C:\runtime`, NodePath: `C:\node.exe`}
	baseReq := PageCommandRequest{
		ProfileID:        "profile-a",
		Selector:         map[string]any{"profileId": "profile-a"},
		LaunchBaseURL:    "http://127.0.0.1:19000",
		LaunchAuthHeader: "Authorization",
		LaunchAuthValue:  "Bearer one",
		ArtifactDir:      `C:\artifacts`,
		Timeout:          15 * time.Second,
	}
	baseKey, err := pageSessionConfigKey(baseState, baseReq)
	if err != nil {
		t.Fatalf("base config key: %v", err)
	}

	tests := []struct {
		name  string
		state RuntimeState
		req   PageCommandRequest
	}{
		{name: "runtime dir", state: RuntimeState{RuntimeDir: `D:\runtime`, NodePath: baseState.NodePath}, req: baseReq},
		{name: "node path", state: RuntimeState{RuntimeDir: baseState.RuntimeDir, NodePath: `D:\node.exe`}, req: baseReq},
		{name: "selector", state: baseState, req: func() PageCommandRequest {
			r := baseReq
			r.Selector = map[string]any{"profileId": "profile-b"}
			return r
		}()},
		{name: "launch base url", state: baseState, req: func() PageCommandRequest { r := baseReq; r.LaunchBaseURL = "http://127.0.0.1:19001"; return r }()},
		{name: "auth header", state: baseState, req: func() PageCommandRequest { r := baseReq; r.LaunchAuthHeader = "X-API-Key"; return r }()},
		{name: "auth value", state: baseState, req: func() PageCommandRequest { r := baseReq; r.LaunchAuthValue = "Bearer two"; return r }()},
		{name: "artifact dir", state: baseState, req: func() PageCommandRequest { r := baseReq; r.ArtifactDir = `D:\artifacts`; return r }()},
		{name: "default timeout", state: baseState, req: func() PageCommandRequest { r := baseReq; r.Timeout = 20 * time.Second; return r }()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := pageSessionConfigKey(tt.state, tt.req)
			if err != nil {
				t.Fatalf("config key: %v", err)
			}
			if key == baseKey {
				t.Fatalf("configuration change did not change page session key")
			}
		})
	}
}

func TestPageSessionExpirationUsesOwnIdleTimeout(t *testing.T) {
	now := time.Now()
	short := &pageSession{lastUsed: now.Add(-2 * time.Minute), idleTimeout: time.Minute}
	long := &pageSession{lastUsed: now.Add(-2 * time.Minute), idleTimeout: 10 * time.Minute}
	if !short.expiredAt(now) {
		t.Fatal("short-idle page session should be expired")
	}
	if long.expiredAt(now) {
		t.Fatal("long-idle page session should not be expired")
	}

	long.setIdleTimeout(30 * time.Second)
	if !long.expiredAt(now) {
		t.Fatal("updated idle timeout was not applied to reused page session")
	}
}

func TestPageSessionReaperIntervalUsesShortestSessionTimeout(t *testing.T) {
	manager := &Manager{pageSessions: map[string]*pageSession{
		"slow": {idleTimeout: 10 * time.Minute, lastUsed: time.Now()},
		"fast": {idleTimeout: 4 * time.Second, lastUsed: time.Now()},
	}}
	if got := manager.pageSessionReaperInterval(); got != time.Second {
		t.Fatalf("pageSessionReaperInterval = %s, want 1s", got)
	}
}

func newBufferedSessionReader(reader io.Reader) *bufio.Reader {
	return bufio.NewReaderSize(reader, 64<<10)
}
