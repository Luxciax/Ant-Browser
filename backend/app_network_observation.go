package backend

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/fsutil"
	"github.com/google/uuid"
)

type BrowserNetworkObservationRequest struct {
	ProfileID string `json:"profileId,omitempty"`
	URL       string `json:"url"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type BrowserNetworkDNSObservation struct {
	Observed      bool     `json:"observed"`
	Host          string   `json:"host,omitempty"`
	AddressCount  int      `json:"addressCount"`
	AddressHashes []string `json:"addressHashes,omitempty"`
	Families      []string `json:"families,omitempty"`
	DurationMs    int64    `json:"durationMs,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type BrowserNetworkTLSObservation struct {
	Observed           bool   `json:"observed"`
	Version            string `json:"version,omitempty"`
	CipherSuite        string `json:"cipherSuite,omitempty"`
	NegotiatedProtocol string `json:"negotiatedProtocol,omitempty"`
	ServerName         string `json:"serverName,omitempty"`
	DurationMs         int64  `json:"durationMs,omitempty"`
	Error              string `json:"error,omitempty"`
}

type BrowserNetworkConnectionObservation struct {
	Observed   bool   `json:"observed"`
	Network    string `json:"network,omitempty"`
	RemoteAddr string `json:"remoteAddr,omitempty"`
	Reused     bool   `json:"reused"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

type BrowserNetworkEvidenceStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type BrowserNetworkObservation struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	ID            string                              `json:"id"`
	ProfileID     string                              `json:"profileId,omitempty"`
	Source        string                              `json:"source"`
	URL           string                              `json:"url"`
	StartedAt     string                              `json:"startedAt"`
	FinishedAt    string                              `json:"finishedAt"`
	TotalMs       int64                               `json:"totalMs"`
	OK            bool                                `json:"ok"`
	StatusCode    int                                 `json:"statusCode,omitempty"`
	HTTPProtocol  string                              `json:"httpProtocol,omitempty"`
	DNS           BrowserNetworkDNSObservation        `json:"dns"`
	Connection    BrowserNetworkConnectionObservation `json:"connection"`
	TLS           BrowserNetworkTLSObservation        `json:"tls"`
	HTTP3         BrowserNetworkEvidenceStatus        `json:"http3"`
	QUIC          BrowserNetworkEvidenceStatus        `json:"quic"`
	JA3           BrowserNetworkEvidenceStatus        `json:"ja3"`
	JA4           BrowserNetworkEvidenceStatus        `json:"ja4"`
	BrowserRoute  BrowserNetworkEvidenceStatus        `json:"browserRoute"`
	ServerSeen    BrowserNetworkEvidenceStatus        `json:"serverSeen"`
	Error         string                              `json:"error,omitempty"`
}

func sanitizeObservationURL(raw string) (*url.URL, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil { return nil, "", fmt.Errorf("invalid observation URL") }
	if parsed.Scheme != "http" && parsed.Scheme != "https" { return nil, "", fmt.Errorf("observation URL must use http or https") }
	if parsed.Hostname() == "" { return nil, "", fmt.Errorf("observation URL host is required") }
	if parsed.User != nil { return nil, "", fmt.Errorf("observation URL must not contain credentials") }
	parsed.Fragment = ""
	parsed.RawQuery = ""
	return parsed, parsed.String(), nil
}

func networkAddressHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:6])
}

func networkAddressFamily(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil { return "unknown" }
	if ip.To4() != nil { return "ipv4" }
	return "ipv6"
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10: return "TLS1.0"
	case tls.VersionTLS11: return "TLS1.1"
	case tls.VersionTLS12: return "TLS1.2"
	case tls.VersionTLS13: return "TLS1.3"
	default:
		if v == 0 { return "" }
		return "0x" + strconv.FormatUint(uint64(v), 16)
	}
}

func uniqueStrings(values []string) []string {
	seen:=map[string]struct{}{};out:=make([]string,0,len(values))
	for _,v:=range values{if _,ok:=seen[v];ok{continue};seen[v]=struct{}{};out=append(out,v)}
	return out
}

func (a *App) networkObservationDir() string {
	return a.resolveAppPath(filepath.Join("data","network-observations"))
}

func (a *App) saveNetworkObservation(value BrowserNetworkObservation) error {
	if err:=os.MkdirAll(a.networkObservationDir(),0o700);err!=nil{return err}
	data,err:=json.MarshalIndent(value,"","  ");if err!=nil{return err}
	return fsutil.AtomicWriteFile(filepath.Join(a.networkObservationDir(),value.ID+".json"),data,0o600)
}

func (a *App) BrowserNetworkObserve(req BrowserNetworkObservationRequest) (*BrowserNetworkObservation,error) {
	target, publicURL, err := sanitizeObservationURL(req.URL)
	if err != nil { return nil, err }
	timeout:=time.Duration(req.TimeoutMs)*time.Millisecond
	if timeout<=0{timeout=15*time.Second};if timeout>60*time.Second{timeout=60*time.Second}
	started:=time.Now().UTC()
	obs:=BrowserNetworkObservation{
		SchemaVersion:1, ID:uuid.NewString(), ProfileID:strings.TrimSpace(req.ProfileID), Source:"host",
		URL:publicURL, StartedAt:started.Format(time.RFC3339Nano),
		HTTP3:BrowserNetworkEvidenceStatus{Status:"not_observed",Reason:"standard host transport probe does not expose independent HTTP/3 proof"},
		QUIC:BrowserNetworkEvidenceStatus{Status:"not_observed",Reason:"no packet capture or QUIC-specific probe was executed"},
		JA3:BrowserNetworkEvidenceStatus{Status:"not_observed",Reason:"client TLS fingerprint was not captured on the wire"},
		JA4:BrowserNetworkEvidenceStatus{Status:"not_observed",Reason:"client TLS fingerprint was not captured on the wire"},
		BrowserRoute:BrowserNetworkEvidenceStatus{Status:"not_observed",Reason:"this probe measures the host transport and does not claim browser proxy routing"},
		ServerSeen:BrowserNetworkEvidenceStatus{Status:"not_observed",Reason:"the target endpoint was not required to provide server-side telemetry"},
	}
	var mu sync.Mutex
	var dnsStart,connectStart,tlsStart time.Time
	trace:=&httptrace.ClientTrace{
		DNSStart:func(info httptrace.DNSStartInfo){mu.Lock();dnsStart=time.Now();obs.DNS.Host=info.Host;mu.Unlock()},
		DNSDone:func(info httptrace.DNSDoneInfo){
			mu.Lock();defer mu.Unlock();obs.DNS.Observed=true
			if !dnsStart.IsZero(){obs.DNS.DurationMs=time.Since(dnsStart).Milliseconds()}
			for _,addr:=range info.Addrs{obs.DNS.AddressHashes=append(obs.DNS.AddressHashes,networkAddressHash(addr.IP.String()));obs.DNS.Families=append(obs.DNS.Families,networkAddressFamily(addr.IP.String()))}
			obs.DNS.AddressCount=len(info.Addrs);obs.DNS.Families=uniqueStrings(obs.DNS.Families)
			if info.Err!=nil{obs.DNS.Error=info.Err.Error()}
		},
		ConnectStart:func(network,addr string){mu.Lock();connectStart=time.Now();obs.Connection.Network=network;mu.Unlock()},
		ConnectDone:func(network,addr string,e error){
			mu.Lock();defer mu.Unlock();obs.Connection.Observed=true;obs.Connection.Network=network;obs.Connection.RemoteAddr=addr
			if !connectStart.IsZero(){obs.Connection.DurationMs=time.Since(connectStart).Milliseconds()}
			if e!=nil{obs.Connection.Error=e.Error()}
		},
		GotConn:func(info httptrace.GotConnInfo){mu.Lock();obs.Connection.Observed=true;obs.Connection.Reused=info.Reused;if info.Conn!=nil{obs.Connection.RemoteAddr=info.Conn.RemoteAddr().String()};mu.Unlock()},
		TLSHandshakeStart:func(){mu.Lock();tlsStart=time.Now();mu.Unlock()},
		TLSHandshakeDone:func(state tls.ConnectionState,e error){
			mu.Lock();defer mu.Unlock();obs.TLS.Observed=true;obs.TLS.ServerName=state.ServerName
			obs.TLS.Version=tlsVersionName(state.Version);obs.TLS.CipherSuite=tls.CipherSuiteName(state.CipherSuite);obs.TLS.NegotiatedProtocol=state.NegotiatedProtocol
			if !tlsStart.IsZero(){obs.TLS.DurationMs=time.Since(tlsStart).Milliseconds()};if e!=nil{obs.TLS.Error=e.Error()}
		},
	}
	ctx,cancel:=context.WithTimeout(context.Background(),timeout);defer cancel()
	ctx=httptrace.WithClientTrace(ctx,trace)
	transport:=&http.Transport{Proxy:http.ProxyFromEnvironment,ForceAttemptHTTP2:true,DisableKeepAlives:true,TLSClientConfig:&tls.Config{MinVersion:tls.VersionTLS12}}
	client:=&http.Client{Transport:transport}
	request,err:=http.NewRequestWithContext(ctx,http.MethodGet,target.String(),nil)
	if err!=nil{return nil,err}
	request.Header.Set("User-Agent","AntBrowser-NetworkObservation/1")
	response,requestErr:=client.Do(request)
	if response!=nil{
		obs.StatusCode=response.StatusCode;obs.HTTPProtocol=response.Proto
		_,_=io.Copy(io.Discard,io.LimitReader(response.Body,64<<10));_ = response.Body.Close()
		obs.OK=response.StatusCode>=200&&response.StatusCode<500
	}
	if requestErr!=nil{obs.Error=requestErr.Error()}
	finished:=time.Now().UTC();obs.FinishedAt=finished.Format(time.RFC3339Nano);obs.TotalMs=finished.Sub(started).Milliseconds()
	if err:=a.saveNetworkObservation(obs);err!=nil{return &obs,fmt.Errorf("network observation completed but evidence persistence failed: %w",err)}
	if requestErr!=nil{return &obs,requestErr}
	return &obs,nil
}

func (a *App) BrowserNetworkObservationGet(id string)(*BrowserNetworkObservation,error){
	id=strings.TrimSpace(id);if id==""||strings.ContainsAny(id,"/\\"){return nil,fmt.Errorf("observation id is invalid")}
	data,err:=os.ReadFile(filepath.Join(a.networkObservationDir(),id+".json"));if err!=nil{return nil,err}
	var out BrowserNetworkObservation;if err:=json.Unmarshal(data,&out);err!=nil{return nil,err};return &out,nil
}
