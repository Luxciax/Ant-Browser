package diagnostics

func networkTimingEvidence(values map[string]float64) map[string]any {
	out := map[string]any{"available": len(values) > 0}
	for name, pair := range map[string][2]string{"dns_ms": {"dnsStart", "dnsEnd"}, "connect_ms": {"connectStart", "connectEnd"}, "tls_ms": {"sslStart", "sslEnd"}, "send_ms": {"sendStart", "sendEnd"}, "headers_ms": {"sendEnd", "receiveHeadersEnd"}} {
		start, a := values[pair[0]]
		end, b := values[pair[1]]
		if a && b && start >= 0 && end >= start {
			out[name] = end - start
		} else {
			out[name] = nil
		}
	}
	return out
}
