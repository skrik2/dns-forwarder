package core

import (
	"context"
	"log"
	"net/netip"

	"df/conf"

	UP "github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
)

var (
	upstreamClients []UP.Upstream
	onceInit        = false
)

func Init() {
	if onceInit {
		return
	}

	cfg := conf.Info()

	opts := &UP.Options{
		HTTPVersions: []UP.HTTPVersion{
			UP.HTTPVersion3,
			UP.HTTPVersion2,
			UP.HTTPVersion11,
		},
	}

	bootstrapIP, err := netip.ParseAddr(cfg.Options.Bootstrap)
	if err != nil {
		log.Fatalf("[fatal] invalid bootstrap IP: %v", err)
	}

	opts.Bootstrap = &singleIPResolver{ip: bootstrapIP}

	// build all upstreams
	for _, addr := range cfg.Upstream {
		up, err := UP.AddressToUpstream(addr, opts)
		if err != nil {
			log.Fatalf("[fatal] invalid upstream %s: %v", addr, err)
		}
		upstreamClients = append(upstreamClients, up)
	}

	onceInit = true
}

func Core(req *dns.Msg) (*dns.Msg, error) {
	if upstreamClients == nil {
		log.Fatalf("[fatal] upstream not initialized")
	}
	// TODO: Block

	// TODO: Subnet

	resp, fastestUp, err := UP.ExchangeParallel(upstreamClients, req)
	if err != nil {
		return nil, err
	}
	// TODO: fastesUp
	_ = fastestUp

	// TODO: TTL
	return resp, nil
}

type singleIPResolver struct {
	ip netip.Addr
}

var _ UP.Resolver = (*singleIPResolver)(nil)

func (s *singleIPResolver) LookupNetIP(_ context.Context, _ string, _ string) (addrs []netip.Addr, err error) {
	return []netip.Addr{s.ip}, nil
}
