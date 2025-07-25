package resolver

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	maxRecursionDepth = 30
	queryTimeout      = 15 * time.Second
)

var rootServerNames = []string{
	"a.root-servers.net",
	"b.root-servers.net",
	"c.root-servers.net",
	"d.root-servers.net",
	"e.root-servers.net",
	"f.root-servers.net",
	"g.root-servers.net",
	"h.root-servers.net",
	"i.root-servers.net",
	"j.root-servers.net",
	"k.root-servers.net",
	"l.root-servers.net",
	"m.root-servers.net",
}

var (
	rootServers     []string
	rootServersOnce sync.Once
)

type Resolver struct {
	client dns.Client
}

type Result struct {
	Answer        []dns.RR
	Authority     []dns.RR
	Additional    []dns.RR
	Rcode         int
	Authoritative bool
}

func New() *Resolver {
	return &Resolver{
		client: dns.Client{
			Timeout: queryTimeout,
			Dialer: &net.Dialer{
				Timeout: queryTimeout,
			},
		},
	}
}

func getRootServers() []string {
	rootServersOnce.Do(func() {
		rootServers = resolveRootServers()
	})
	return rootServers
}

func resolveRootServers() []string {
	var servers []string

	for _, name := range rootServerNames {
		ips, err := net.LookupIP(name)
		if err != nil {
			continue
		}

		for _, ip := range ips {
			servers = append(servers, net.JoinHostPort(ip.String(), "53"))
		}
	}

	return servers
}

func (r *Resolver) Resolve(domain string, qtype uint16) (*Result, error) {
	domain = dns.Fqdn(domain)
	return r.resolveRecursive(domain, qtype, getRootServers(), 0)
}

func (r *Resolver) resolveRecursive(domain string, qtype uint16, nameservers []string, depth int) (*Result, error) {
	if depth > maxRecursionDepth {
		return nil, fmt.Errorf("maximum recursion depth exceeded")
	}

	if len(nameservers) == 0 {
		return nil, fmt.Errorf("no nameservers available")
	}

	for _, ns := range nameservers {
		result, err := r.queryNameserver(ns, domain, qtype)
		if err != nil {
			continue
		}

		if result.Rcode != dns.RcodeSuccess {
			if result.Rcode == dns.RcodeNameError {
				return result, nil
			}
			continue
		}

		if len(result.Answer) > 0 {
			result.Answer = r.followCNAME(result.Answer, qtype, depth)
			return result, nil
		}

		if len(result.Authority) > 0 {
			nsRecords := r.extractNSRecords(result.Authority)
			if len(nsRecords) == 0 {
				continue
			}

			nextNS := r.resolveNameservers(nsRecords, result.Additional)
			if len(nextNS) == 0 {
				continue
			}

			return r.resolveRecursive(domain, qtype, nextNS, depth+1)
		}
	}

	return nil, fmt.Errorf("no answer found for %s", domain)
}

func (r *Resolver) queryNameserver(nameserver, domain string, qtype uint16) (*Result, error) {
	if !strings.Contains(nameserver, ":") {
		nameserver = nameserver + ":53"
	}

	m := new(dns.Msg)
	m.SetQuestion(domain, qtype)
	m.RecursionDesired = false

	resp, _, err := r.client.Exchange(m, nameserver)
	if err != nil {
		return nil, err
	}

	return &Result{
		Answer:        resp.Answer,
		Authority:     resp.Ns,
		Additional:    resp.Extra,
		Rcode:         resp.Rcode,
		Authoritative: resp.Authoritative,
	}, nil
}

func (r *Resolver) followCNAME(answers []dns.RR, originalType uint16, depth int) []dns.RR {
	result := make([]dns.RR, 0, len(answers))

	for _, rr := range answers {
		result = append(result, rr)

		if cname, ok := rr.(*dns.CNAME); ok && originalType != dns.TypeCNAME {
			cnameResult, err := r.resolveRecursive(cname.Target, originalType, getRootServers(), depth+1)
			if err == nil && len(cnameResult.Answer) > 0 {
				result = append(result, cnameResult.Answer...)
			}
		}
	}

	return result
}

func (r *Resolver) extractNSRecords(authority []dns.RR) []string {
	var nsRecords []string
	for _, rr := range authority {
		if ns, ok := rr.(*dns.NS); ok {
			nsRecords = append(nsRecords, dns.Fqdn(ns.Ns))
		}
	}
	return nsRecords
}

func (r *Resolver) resolveNameservers(nsRecords []string, additional []dns.RR) []string {
	var nameservers []string

	additionalMap := make(map[string][]net.IP)
	for _, rr := range additional {
		switch rec := rr.(type) {
		case *dns.A:
			additionalMap[rec.Hdr.Name] = append(additionalMap[rec.Hdr.Name], rec.A)
		case *dns.AAAA:
			additionalMap[rec.Hdr.Name] = append(additionalMap[rec.Hdr.Name], rec.AAAA)
		}
	}

	for _, nsName := range nsRecords {
		if ips, found := additionalMap[nsName]; found {
			for _, ip := range ips {
				nameservers = append(nameservers, net.JoinHostPort(ip.String(), "53"))
			}
		}
	}

	return nameservers
}
