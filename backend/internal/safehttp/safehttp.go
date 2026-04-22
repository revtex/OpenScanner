// Package safehttp provides an *http.Client with SSRF defenses:
// redirects are disabled and connections to private / loopback / link-local
// / multicast addresses are rejected unless the operator opts in via
// OPENSCANNER_ALLOW_INTERNAL_HTTP=1.
//
// Deployments that run the go-whisper sidecar on localhost MUST set the
// allow-internal env var — otherwise transcription HTTP calls will fail.
package safehttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// envAllowInternal controls whether private/loopback/link-local/multicast
// destinations are permitted. Intended for homelab deployments and tests.
const envAllowInternal = "OPENSCANNER_ALLOW_INTERNAL_HTTP"

// ErrBlockedAddress is returned when a dial target resolves to an address
// that is blocked by the SSRF allow-list.
var ErrBlockedAddress = errors.New("safehttp: blocked internal address")

var (
	allowInternalOnce sync.Once
	allowInternalVal  bool
)

// AllowInternal reports whether loopback / private addresses are permitted.
// Cached for the lifetime of the process after the first call.
func AllowInternal() bool {
	allowInternalOnce.Do(func() {
		v := strings.TrimSpace(os.Getenv(envAllowInternal))
		allowInternalVal = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	return allowInternalVal
}

// isBlockedIP reports whether ip falls in a disallowed range when
// AllowInternal() is false.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	return false
}

// safeDialContext is a net.Dialer.DialContext wrapper that rejects blocked
// addresses. It resolves the host, filters, and dials the first allowed IP.
func safeDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if AllowInternal() {
			return dialer.DialContext(ctx, network, addr)
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// Resolve all A/AAAA records so we can reject if any resolve to
		// a blocked range (DNS-rebinding defense).
		resolver := net.DefaultResolver
		ips, err := resolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, ErrBlockedAddress
		}
		for _, ip := range ips {
			if isBlockedIP(ip) {
				return nil, ErrBlockedAddress
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}

// Client returns an *http.Client that disables redirect following and
// refuses to connect to private / loopback / link-local / multicast
// addresses unless OPENSCANNER_ALLOW_INTERNAL_HTTP is set.
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           safeDialContext(dialer),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
