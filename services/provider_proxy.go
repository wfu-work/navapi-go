package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"navapi-go/domains"

	xproxy "golang.org/x/net/proxy"
)

const (
	providerProxyTypeHTTP   = "http"
	providerProxyTypeHTTPS  = "https"
	providerProxyTypeSOCKS5 = "socks5"

	streamDialTimeout           = 15 * time.Second
	streamTLSHandshakeTimeout   = 15 * time.Second
	streamResponseHeaderTimeout = 10 * time.Minute
	providerTransportCacheLimit = 64
	providerTransportCacheTTL   = 10 * time.Minute
)

type cachedProviderTransport struct {
	transport *http.Transport
	createdAt time.Time
}

var providerTransportCache = struct {
	sync.Mutex
	items map[string]cachedProviderTransport
}{items: make(map[string]cachedProviderTransport)}

func normalizeProviderProxyConfig(provider *domains.VendorMeta) {
	provider.ProxyURL = strings.TrimSpace(provider.ProxyURL)
	provider.ProxyUsername = strings.TrimSpace(provider.ProxyUsername)
	provider.ProxyPassword = strings.TrimSpace(provider.ProxyPassword)
	provider.ProxyType = normalizeProviderProxyType(provider.ProxyType, provider.ProxyURL)
}

func validateProviderProxyConfig(provider *domains.VendorMeta) error {
	if provider == nil || !provider.ProxyEnabled {
		return nil
	}
	_, err := providerProxyURL(provider)
	return err
}

func normalizeProviderProxyType(value string, rawURL string) string {
	if scheme := proxySchemeFromURL(rawURL); scheme != "" {
		return scheme
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case providerProxyTypeHTTPS:
		return providerProxyTypeHTTPS
	case providerProxyTypeSOCKS5:
		return providerProxyTypeSOCKS5
	default:
		return providerProxyTypeHTTP
	}
}

func proxySchemeFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case providerProxyTypeHTTP:
		return providerProxyTypeHTTP
	case providerProxyTypeHTTPS:
		return providerProxyTypeHTTPS
	case providerProxyTypeSOCKS5, "socks5h":
		return providerProxyTypeSOCKS5
	default:
		return ""
	}
}

func providerProxyURL(provider *domains.VendorMeta) (*url.URL, error) {
	if provider == nil || !provider.ProxyEnabled {
		return nil, nil
	}
	rawURL := strings.TrimSpace(provider.ProxyURL)
	if rawURL == "" {
		return nil, errors.New("proxy url is required")
	}
	proxyType := normalizeProviderProxyType(provider.ProxyType, rawURL)
	if !strings.Contains(rawURL, "://") {
		rawURL = proxyType + "://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return nil, errors.New("proxy url is invalid")
	}
	switch strings.ToLower(u.Scheme) {
	case providerProxyTypeHTTP, providerProxyTypeHTTPS:
	case providerProxyTypeSOCKS5, "socks5h":
		u.Scheme = providerProxyTypeSOCKS5
	default:
		return nil, errors.New("proxy type only supports http, https and socks5")
	}
	username := strings.TrimSpace(provider.ProxyUsername)
	password := strings.TrimSpace(provider.ProxyPassword)
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}
	return u, nil
}

func providerProxyEnabled(provider *domains.VendorMeta) bool {
	return provider != nil && provider.ProxyEnabled
}

func providerHTTPClient(provider *domains.VendorMeta, timeout time.Duration) (*http.Client, error) {
	if !providerProxyEnabled(provider) {
		return &http.Client{Timeout: timeout}, nil
	}
	transport, err := cachedProviderTransportFor(provider, false)
	if err != nil {
		return nil, err
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

func providerStreamHTTPClient(provider *domains.VendorMeta) (*http.Client, error) {
	transport, err := cachedProviderTransportFor(provider, true)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport}, nil
}

func newStreamHTTPClient() *http.Client {
	return &http.Client{Transport: cloneStreamTransport()}
}

func (s RelayService) clientForProvider(provider *domains.VendorMeta) (*http.Client, error) {
	if !providerProxyEnabled(provider) {
		return s.client, nil
	}
	return providerHTTPClient(provider, s.client.Timeout)
}

func (s RelayService) streamClientForProvider(provider *domains.VendorMeta) (*http.Client, error) {
	if !providerProxyEnabled(provider) {
		if s.streamClient != nil {
			return s.streamClient, nil
		}
		// Tests and explicitly constructed RelayService values may only provide
		// one client. Production always uses the dedicated streaming client.
		if s.client != nil {
			return s.client, nil
		}
		return newStreamHTTPClient(), nil
	}
	return providerStreamHTTPClient(provider)
}

func providerTransport(provider *domains.VendorMeta) (*http.Transport, error) {
	return providerTransportWithMode(provider, false)
}

func cachedProviderTransportFor(provider *domains.VendorMeta, streaming bool) (*http.Transport, error) {
	key := providerTransportKey(provider, streaming)
	now := time.Now()
	providerTransportCache.Lock()
	if cached, ok := providerTransportCache.items[key]; ok && now.Sub(cached.createdAt) < providerTransportCacheTTL {
		providerTransportCache.Unlock()
		return cached.transport, nil
	}
	providerTransportCache.Unlock()

	transport, err := providerTransportWithMode(provider, streaming)
	if err != nil {
		return nil, err
	}
	providerTransportCache.Lock()
	if previous, ok := providerTransportCache.items[key]; ok {
		previous.transport.CloseIdleConnections()
	}
	providerTransportCache.items[key] = cachedProviderTransport{transport: transport, createdAt: now}
	if len(providerTransportCache.items) > providerTransportCacheLimit {
		keys := make([]string, 0, len(providerTransportCache.items))
		for itemKey := range providerTransportCache.items {
			keys = append(keys, itemKey)
		}
		sort.Slice(keys, func(i, j int) bool {
			return providerTransportCache.items[keys[i]].createdAt.Before(providerTransportCache.items[keys[j]].createdAt)
		})
		for _, evictKey := range keys[:len(keys)-providerTransportCacheLimit] {
			providerTransportCache.items[evictKey].transport.CloseIdleConnections()
			delete(providerTransportCache.items, evictKey)
		}
	}
	providerTransportCache.Unlock()
	return transport, nil
}

func providerTransportKey(provider *domains.VendorMeta, streaming bool) string {
	if provider == nil {
		return "default"
	}
	raw := strings.Join([]string{
		provider.Guid,
		provider.ProxyType,
		provider.ProxyURL,
		provider.ProxyUsername,
		provider.ProxyPassword,
		boolString(provider.ProxyEnabled),
		boolString(streaming),
	}, "\x00")
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func providerTransportWithMode(provider *domains.VendorMeta, streaming bool) (*http.Transport, error) {
	transport := cloneDefaultTransport()
	if streaming {
		transport = cloneStreamTransport()
	}
	proxyURL, err := providerProxyURL(provider)
	if err != nil {
		return nil, err
	}
	if proxyURL == nil {
		return transport, nil
	}
	switch strings.ToLower(proxyURL.Scheme) {
	case providerProxyTypeHTTP, providerProxyTypeHTTPS:
		transport.Proxy = http.ProxyURL(proxyURL)
	case providerProxyTypeSOCKS5:
		dialer, err := socks5Dialer(proxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
			if streaming {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, streamDialTimeout)
				defer cancel()
			}
			return dialWithContext(ctx, dialer, network, address)
		}
	default:
		return nil, errors.New("proxy type only supports http, https and socks5")
	}
	return transport, nil
}

func cloneDefaultTransport() *http.Transport {
	var transport *http.Transport
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = defaultTransport.Clone()
	} else {
		transport = &http.Transport{}
	}
	transport.MaxIdleConns = 200
	transport.MaxIdleConnsPerHost = 50
	transport.IdleConnTimeout = 90 * time.Second
	transport.ForceAttemptHTTP2 = true
	return transport
}

func cloneStreamTransport() *http.Transport {
	transport := cloneDefaultTransport()
	transport.DialContext = (&net.Dialer{
		Timeout:   streamDialTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = streamTLSHandshakeTimeout
	transport.ResponseHeaderTimeout = streamResponseHeaderTimeout
	return transport
}

func socks5Dialer(proxyURL *url.URL) (xproxy.Dialer, error) {
	var auth *xproxy.Auth
	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		if username != "" {
			auth = &xproxy.Auth{User: username, Password: password}
		}
	}
	return xproxy.SOCKS5("tcp", proxyURL.Host, auth, xproxy.Direct)
}

func dialWithContext(ctx context.Context, dialer xproxy.Dialer, network string, address string) (net.Conn, error) {
	type contextDialer interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
	if dialerWithContext, ok := dialer.(contextDialer); ok {
		return dialerWithContext.DialContext(ctx, network, address)
	}
	type dialResult struct {
		conn net.Conn
		err  error
	}
	resultCh := make(chan dialResult, 1)
	go func() {
		conn, err := dialer.Dial(network, address)
		resultCh <- dialResult{conn: conn, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.conn, result.err
	}
}
