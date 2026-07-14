package services

import (
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"
)

type UpstreamTiming struct {
	RequestBodyBytes     int64
	DNSLookupTimeMs      int64
	ConnectTimeMs        int64
	TLSHandshakeTimeMs   int64
	RequestWriteTimeMs   int64
	ResponseHeaderTimeMs int64
	UpstreamTotalTimeMs  int64
	ConnectionReused     bool
	AttemptCount         int
}

type upstreamRequestTrace struct {
	mu sync.Mutex

	start              time.Time
	requestBodyBytes   int64
	dnsStart           time.Time
	connectStart       time.Time
	tlsStart           time.Time
	dnsLookupTime      time.Duration
	connectTime        time.Duration
	tlsHandshakeTime   time.Duration
	requestWriteTime   time.Duration
	responseHeaderTime time.Duration
	connectionReused   bool
}

func newUpstreamRequestTrace(start time.Time, requestBodyBytes int64) *upstreamRequestTrace {
	return &upstreamRequestTrace{start: start, requestBodyBytes: requestBodyBytes}
}

func (t *upstreamRequestTrace) ClientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			t.mu.Lock()
			t.dnsStart = time.Now()
			t.mu.Unlock()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			t.mu.Lock()
			if !t.dnsStart.IsZero() {
				t.dnsLookupTime += time.Since(t.dnsStart)
				t.dnsStart = time.Time{}
			}
			t.mu.Unlock()
		},
		ConnectStart: func(_, _ string) {
			t.mu.Lock()
			t.connectStart = time.Now()
			t.mu.Unlock()
		},
		ConnectDone: func(_, _ string, _ error) {
			t.mu.Lock()
			if !t.connectStart.IsZero() {
				t.connectTime += time.Since(t.connectStart)
				t.connectStart = time.Time{}
			}
			t.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			t.mu.Lock()
			t.tlsStart = time.Now()
			t.mu.Unlock()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			t.mu.Lock()
			if !t.tlsStart.IsZero() {
				t.tlsHandshakeTime += time.Since(t.tlsStart)
				t.tlsStart = time.Time{}
			}
			t.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			t.connectionReused = info.Reused
			t.mu.Unlock()
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			t.mu.Lock()
			t.requestWriteTime = time.Since(t.start)
			t.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			if t.responseHeaderTime <= 0 {
				t.responseHeaderTime = time.Since(t.start)
			}
			t.mu.Unlock()
		},
	}
}

func (t *upstreamRequestTrace) Snapshot(total time.Duration) UpstreamTiming {
	t.mu.Lock()
	defer t.mu.Unlock()
	return UpstreamTiming{
		RequestBodyBytes:     t.requestBodyBytes,
		DNSLookupTimeMs:      t.dnsLookupTime.Milliseconds(),
		ConnectTimeMs:        t.connectTime.Milliseconds(),
		TLSHandshakeTimeMs:   t.tlsHandshakeTime.Milliseconds(),
		RequestWriteTimeMs:   t.requestWriteTime.Milliseconds(),
		ResponseHeaderTimeMs: t.responseHeaderTime.Milliseconds(),
		UpstreamTotalTimeMs:  total.Milliseconds(),
		ConnectionReused:     t.connectionReused,
	}
}
