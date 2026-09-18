package backend

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeCDPJSONReader struct {
	messages []string
	err      error
	index    int
}

func (r *fakeCDPJSONReader) ReadJSON(v interface{}) error {
	if r.index >= len(r.messages) {
		if r.err != nil {
			return r.err
		}
		return errors.New("no more messages")
	}
	message := r.messages[r.index]
	r.index++
	return json.Unmarshal([]byte(message), v)
}

func TestReadCDPResponseForIDSkipsEventsAndOtherResponses(t *testing.T) {
	reader := &fakeCDPJSONReader{messages: []string{
		`{"method":"Network.requestWillBeSent","params":{"requestId":"1"}}`,
		`{"id":2,"result":{"ignored":true}}`,
		`{"id":1,"result":{"cookies":[{"name":"session"}]}}`,
	}}
	response, err := readCDPResponseForID(reader, 1)
	if err != nil {
		t.Fatalf("readCDPResponseForID returned error: %v", err)
	}
	if response.Id != 1 {
		t.Fatalf("response id = %d, want 1", response.Id)
	}
	if _, ok := response.Result["cookies"]; !ok {
		t.Fatalf("response result = %#v, want cookies", response.Result)
	}
	if reader.index != 3 {
		t.Fatalf("reader consumed %d messages, want 3", reader.index)
	}
}

func TestReadCDPResponseForIDReturnsMatchingProtocolError(t *testing.T) {
	reader := &fakeCDPJSONReader{messages: []string{
		`{"method":"Target.targetInfoChanged","params":{}}`,
		`{"id":7,"error":{"message":"permission denied"}}`,
	}}
	response, err := readCDPResponseForID(reader, 7)
	if err != nil {
		t.Fatalf("readCDPResponseForID returned transport error: %v", err)
	}
	if response.Error == nil || response.Error.Message != "permission denied" {
		t.Fatalf("response error = %#v, want permission denied", response.Error)
	}
}

func TestReadCDPResponseForIDPropagatesReadFailureBeforeMatch(t *testing.T) {
	expected := errors.New("socket closed")
	reader := &fakeCDPJSONReader{
		messages: []string{`{"method":"Runtime.consoleAPICalled","params":{}}`},
		err:      expected,
	}
	_, err := readCDPResponseForID(reader, 1)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v, want %v", err, expected)
	}
}

func TestFormatNetscapeCookiesPreservesHttpOnlySecureSessionAndSubdomain(t *testing.T) {
	cookies := []CookieInfo{
		{
			Name:     "session",
			Value:    "abc123",
			Domain:   ".example.com",
			Path:     "/",
			Expires:  -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: "Lax",
		},
		{
			Name:    "pref",
			Value:   "dark",
			Domain:  "example.com",
			Path:    "/settings",
			Expires: 1893456000,
		},
	}

	got := formatNetscapeCookies(cookies)
	wantLines := []string{
		"#HttpOnly_.example.com\tTRUE\t/\tTRUE\t0\tsession\tabc123",
		"example.com\tFALSE\t/settings\tFALSE\t1893456000\tpref\tdark",
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted cookies = %q, missing %q", got, want)
		}
	}
}
