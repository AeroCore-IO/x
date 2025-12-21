package dtls

import (
	"context"
	"net"
	"sync"
	"syscall"
	"time"

	piondtls "github.com/pion/dtls/v2"
	"golang.org/x/sys/unix"
)

var (
	// Simple IP cache to avoid frequent DNS lookups
	ipCache sync.Map // map[string]cachedIP

	// Global session store for DTLS resumption
	sessionStore = &SimpleSessionStore{}
)

type SimpleSessionStore struct {
	m sync.Map
}

func (s *SimpleSessionStore) Set(key []byte, session piondtls.Session) error {
	s.m.Store(string(key), session)
	return nil
}

func (s *SimpleSessionStore) Get(key []byte) (piondtls.Session, error) {
	if v, ok := s.m.Load(string(key)); ok {
		return v.(piondtls.Session), nil
	}
	return piondtls.Session{}, nil
}

func (s *SimpleSessionStore) Del(key []byte) error {
	s.m.Delete(string(key))
	return nil
}

type cachedIP struct {
	ip        string
	expiresAt time.Time
}

func resolve(ctx context.Context, addr string, mark int) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, nil // Already an IP or invalid
	}

	// Check cache
	if v, ok := ipCache.Load(host); ok {
		c := v.(cachedIP)
		if time.Now().Before(c.expiresAt) {
			return net.JoinHostPort(c.ip, port), nil
		}
		ipCache.Delete(host)
	}

	// If it's already an IP, don't resolve
	if net.ParseIP(host) != nil {
		return addr, nil
	}

	// Resolve
	resolver := &net.Resolver{
		PreferGo: true,
	}
	if mark > 0 {
		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Control: func(network, address string, c syscall.RawConn) error {
					return c.Control(func(fd uintptr) {
						unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, mark)
					})
				},
			}
			return d.DialContext(ctx, network, address)
		}
	}

	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return "", err
	}

	if len(ips) == 0 {
		return "", &net.DNSError{Err: "no such host", Name: host}
	}

	ip := ips[0].String()
	ipCache.Store(host, cachedIP{
		ip:        ip,
		expiresAt: time.Now().Add(5 * time.Minute), // Cache for 5 minutes
	})

	return net.JoinHostPort(ip, port), nil
}
