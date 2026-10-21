// Package web implements the daemon-side `meno web search|fetch` service.
// It is defense in depth for the model's network access, not a sandbox: the
// Bash tool can still run any command.
package web

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
)

// ErrBlocked marks a destination the policy refuses.
var ErrBlocked = errors.New("destination is not allowed")

// Policy decides which addresses a fetch may reach.
type Policy struct {
	// AllowLocal permits loopback, private and link-local addresses. It is an
	// explicit opt-in; the default refuses them.
	AllowLocal bool
}

var metadata = netip.MustParseAddr("169.254.169.254")

// Check reports why ip must not be contacted, or nil.
func (p Policy) Check(ip netip.Addr) error {
	ip = ip.Unmap()
	if p.AllowLocal && ip != metadata {
		return nil
	}
	switch {
	case ip == metadata:
		return fmt.Errorf("%w: cloud metadata address", ErrBlocked)
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(),
		ip.IsMulticast(), ip.IsUnspecified(), ip.IsInterfaceLocalMulticast():
		return fmt.Errorf("%w: %s is a local or private address", ErrBlocked, ip)
	}
	return nil
}

// Control is a net.Dialer Control hook. It sees the address that is really
// about to be connected, after DNS resolution and for every redirect, so a name
// that resolves to a private address (or changes between lookups) is refused.
func (p Policy) Control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %q is not an IP address", ErrBlocked, host)
	}
	return p.Check(ip)
}

// dialer builds the dialer used for every outgoing connection.
func (p Policy) dialer() *net.Dialer {
	return &net.Dialer{Control: p.Control}
}
