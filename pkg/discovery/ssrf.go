package discovery

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

var (
	// Private and link-local IPv4/IPv6 CIDR blocks for SSRF prevention.
	restrictedCIDRs []*net.IPNet
)

func init() {
	rawCIDRs := []string{
		"0.0.0.0/8",          // Current network (only valid as source address)
		"10.0.0.0/8",         // Private network RFC 1918
		"100.64.0.0/10",      // Shared Address Space RFC 6598
		"127.0.0.0/8",        // Loopback RFC 1122
		"169.254.0.0/16",     // Link-local / AWS & Cloud Metadata service
		"172.16.0.0/12",      // Private network RFC 1918
		"192.0.0.0/24",       // IETF Protocol Assignments
		"192.0.2.0/24",       // TEST-NET-1
		"192.168.0.0/16",     // Private network RFC 1918
		"198.18.0.0/15",      // Benchmarking
		"198.51.100.0/24",    // TEST-NET-2
		"203.0.113.0/24",     // TEST-NET-3
		"224.0.0.0/4",        // Multicast
		"240.0.0.0/4",        // Reserved for future use
		"::/128",             // Unspecified
		"::1/128",            // IPv6 Loopback
		"fc00::/7",           // IPv6 Unique local addresses
		"fe80::/10",          // IPv6 Link-local unicast
		"ff00::/8",           // IPv6 Multicast
	}

	for _, cidr := range rawCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			restrictedCIDRs = append(restrictedCIDRs, ipNet)
		}
	}
}

// IsRestrictedIP returns true if the specified IP falls into private, loopback, or cloud-metadata ranges.
func IsRestrictedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// Normalize IPv4-mapped IPv6
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range restrictedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// NewSafeHTTPClient constructs an http.Client equipped with strict SSRF defense & DNS rebinding protection.
func NewSafeHTTPClient(cfg DiscoveryConfig) *http.Client {
	dialer := &net.Dialer{
		Timeout:   cfg.HTTPTimeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return nil
		},
	}

	transport := &http.Transport{
		Proxy:                 nil, // Disable proxies to prevent proxy traversal bypasses
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   cfg.HTTPTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address format %s: %w", addr, err)
			}

			// In test/local mode if private IPs are allowed, bypass SSRF checks
			if cfg.AllowPrivateIPs {
				return dialer.DialContext(ctx, network, addr)
			}

			// Resolve host IP addresses
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve host %s: %w", host, err)
			}

			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses resolved for %s", host)
			}

			// Validate every resolved IP against restricted SSRF CIDRs
			for _, ip := range ips {
				if IsRestrictedIP(ip) {
					return nil, fmt.Errorf("SSRF protection blocked access to restricted IP %s for host %s", ip.String(), host)
				}
			}

			// Connect to the first safe verified IP (pins IP to prevent TOCTOU DNS rebinding)
			pinnedAddr := net.JoinHostPort(ips[0].String(), port)
			return dialer.DialContext(ctx, network, pinnedAddr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.HTTPTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("stopped after 3 redirects to prevent redirect loops")
			}
			// Enforce HTTPS across redirects unless insecure HTTP is explicitly allowed
			if !cfg.AllowInsecureHTTP && req.URL.Scheme != "https" {
				return fmt.Errorf("insecure redirect to non-HTTPS scheme %s blocked", req.URL.Scheme)
			}
			return nil
		},
	}
}
