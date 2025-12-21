package net

import (
	"context"
	"fmt"
	"net"

	"github.com/go-gost/core/hosts"
	"github.com/go-gost/core/logger"
	"github.com/go-gost/core/resolver"
	ctxvalue "github.com/go-gost/x/ctx"
)

func Resolve(ctx context.Context, network, addr string, r resolver.Resolver, hosts hosts.HostMapper, log logger.Logger) (string, error) {
	if addr == "" {
		return addr, nil
	}

	host, port, _ := net.SplitHostPort(addr)
	if host == "" {
		return addr, nil
	}

	if log == nil {
		log = logger.Default()
	}
	log = log.WithFields(map[string]any{
		"sid": ctxvalue.SidFromContext(ctx),
	})

	if hosts != nil {
		if ips, _ := hosts.Lookup(ctx, network, host); len(ips) > 0 {
			log.Debugf("hit host mapper: %s -> %s", host, ips)
			return net.JoinHostPort(ips[0].String(), port), nil
		}
	}

	if r != nil {
		ips, err := r.Resolve(ctx, network, host)
		if err != nil {
			if err == resolver.ErrInvalid {
				return addr, nil
			}
			log.Error(err)
			// If resolver fails, we should return the error instead of falling back to the original address,
			// otherwise the dialer might try to resolve it again using the system resolver,
			// which might cause a loop if the system resolver is also broken (e.g. routed to TUN).
			// However, if the error is just "no such host", maybe we should let it fail.
			// But wait, if r is NOT nil, it means we configured a custom resolver.
			// If that fails, we probably shouldn't silently fall back.
			// BUT, existing logic returns addr if err == resolver.ErrInvalid.
			// Let's keep it as is for now, but be aware.
		}
		if len(ips) == 0 {
			// If custom resolver returns no IPs, we return error.
			// But if r was nil, we return addr (letting system resolve it).
			// The issue is when r is NOT nil but fails.
			// In our case (Wing), r is usually nil for the main tunnel, so we rely on system resolver.
			return "", fmt.Errorf("resolver: domain %s does not exist", host)
		}
		return net.JoinHostPort(ips[0].String(), port), nil
	}
	return addr, nil
}
