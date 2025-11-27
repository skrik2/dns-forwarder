package upstream

import (
	"io"

	"github.com/miekg/dns"
)

var UpstreamAddrs []string

type Upstream interface {
	Exchange(req *dns.Msg) (resp *dns.Msg, err error)

	Address() (addr string)

	io.Closer
}
