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

func newBufferedSessionReader(reader io.Reader) *bufio.Reader {
	return bufio.NewReaderSize(reader, 64<<10)
}
