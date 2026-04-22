// Package safehttp provides an *http.Client with SSRF defenses:
// redirects are disabled, response body size is capped, and timeouts are
// always enforced.
//
// OpenScanner is a self-hosted homelab tool — almost every legitimate
// downstream URL (go-whisper, rdio-scanner, Home Assistant, etc.) sits on
// a private / LAN address. Blocking those by default would break normal
// deployments, so private / loopback / link-local destinations are ALLOWED
// by default. Operators running OpenScanner on a public network with
// untrusted admins can opt in to SSRF-style blocking with
// OPENSCANNER_BLOCK_INTERNAL_HTTP=1, which rejects RFC1918 / loopback /
// link-local / multicast / unspecified targets (DNS-rebinding aware).
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

// envBlockInternal, when truthy, opts in to SSRF-style blocking of
// private/loopback/link-local/multicast destinations. Default: unset
// (all destinations permitted — appropriate for homelab deployments).
const envBlockInternal = "OPENSCANNER_BLOCK_INTERNAL_HTTP"

// ErrBlockedAddress is returned when a dial target resolves to an address
// that is blocked by the SSRF allow-list.
var ErrBlockedAddress = errors.New("safehttp: blocked internal address")

var (
	blockInternalOnce sync.Once
	blockInternalVal  bool
)

// BlockInternal reports whether loopback / private / link-local addresses
// should be rejected. Default false (homelab-friendly). Cached for the
// lifetime of the process after the first call.
func BlockInternal() bool {
	blockInternalOnce.Do(func() {
		v := strings.TrimSpace(os.Getenv(envBlockInternal))
		blockInternalVal = v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	})
	return blockInternalVal
}

// isBlockedIP reports whether ip falls in a disallowed range when
// BlockInternal() is true.
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
		if !BlockInternal() {
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
// enforces the provided timeout. When OPENSCANNER_BLOCK_INTERNAL_HTTP is
// set, connections to private / loopback / link-local / multicast
// addresses are rejected at dial time.
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
