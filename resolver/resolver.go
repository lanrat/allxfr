// Package resolver provides a recursive DNS resolver that handles missing glue records
// and implements caching for improved performance.
package resolver

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	maxRecursionDepth   = 30
	defaultQueryTimeout = 15 * time.Second
)

// RootServerNames contains the hostnames of the DNS root servers.
// These are the authoritative nameservers for the root zone of the DNS hierarchy.
var RootServerNames = []string{
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

// Resolver implements a recursive DNS resolver with caching and missing glue record handling.
// It follows the DNS resolution process by starting from root servers and following
// referrals until it reaches an authoritative answer.
type Resolver struct {
	client dns.Client
	cache  *dnsCache
}

// Result represents the outcome of a DNS query operation.
// It contains the response sections and metadata from the DNS resolution process.
type Result struct {
	Answer        []dns.RR // Resource records that directly answer the query
	Authority     []dns.RR // Authority section containing NS records for delegation
	Additional    []dns.RR // Additional section containing glue records
	Rcode         int      // DNS response code (e.g., NOERROR, NXDOMAIN)
	Authoritative bool     // Whether the response came from an authoritative server
}

// New creates a new DNS resolver with default cache size.
// The resolver is configured with appropriate timeouts and will handle
// missing glue records automatically.
func New() *Resolver {
	return NewWithCacheSize(defaultCacheSize, defaultQueryTimeout)
}

// NewWithTimeout creates a new DNS resolver with the specified timeout.
// The timeout parameter determines how long to wait for DNS responses.
func NewWithTimeout(timeout time.Duration) *Resolver {
	return NewWithCacheSize(defaultCacheSize, timeout)
}

// NewWithCacheSize creates a new DNS resolver with the specified cache size.
// The cacheSize parameter determines how many DNS responses can be cached
// using an LRU eviction policy.
func NewWithCacheSize(cacheSize int, timeout time.Duration) *Resolver {
	return &Resolver{
		client: dns.Client{
			Timeout: timeout,
			Dialer: &net.Dialer{
				Timeout: timeout,
			},
		},
		cache: newDNSCache(cacheSize),
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

	for _, name := range RootServerNames {
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

// Resolve performs a recursive DNS lookup for the given domain and query type.
// It starts from the root servers and follows the DNS delegation chain until
// it finds an authoritative answer. Results are cached based on their TTL values.
//
// The domain parameter should be a valid domain name (will be converted to FQDN).
// The qtype parameter specifies the DNS record type (e.g., dns.TypeA, dns.TypeAAAA).
//
// Returns a Result containing the DNS response sections and metadata, or an error
// if the resolution fails.
func (r *Resolver) Resolve(domain string, qtype uint16) (*Result, error) {
	domain = dns.Fqdn(domain)

	cacheKey := r.makeCacheKey(domain, qtype)
	if cached, found := r.cache.get(cacheKey); found {
		return cached, nil
	}

	result, err := r.resolveRecursive(domain, qtype, getRootServers(), 0)
	if err != nil {
		return nil, err
	}

	if result != nil && result.Rcode == dns.RcodeSuccess {
		ttl := r.calculateTTL(result)
		if ttl > 0 {
			r.cache.put(cacheKey, result, ttl)
		}
	}

	return result, nil
}

// ResolveAll performs a recursive DNS lookup for the given domain and query type,
// querying ALL authoritative nameservers and returning the union of all answers.
// Unlike Resolve which returns after the first successful response, ResolveAll
// queries every authoritative nameserver at each level and merges all results.
//
// The domain parameter should be a valid domain name (will be converted to FQDN).
// The qtype parameter specifies the DNS record type (e.g., dns.TypeA, dns.TypeAAAA).
//
// Returns a Result containing the merged DNS response sections from all nameservers,
// or an error if the resolution fails.
func (r *Resolver) ResolveAll(domain string, qtype uint16) (*Result, error) {
	domain = dns.Fqdn(domain)

	cacheKey := r.makeCacheKey(domain+"_ALL", qtype)
	if cached, found := r.cache.get(cacheKey); found {
		return cached, nil
	}

	result, err := r.resolveRecursiveAll(domain, qtype, getRootServers(), 0)
	if err != nil {
		return nil, err
	}

	if result != nil && result.Rcode == dns.RcodeSuccess {
		ttl := r.calculateTTL(result)
		if ttl > 0 {
			r.cache.put(cacheKey, result, ttl)
		}
	}

	return result, nil
}

// LookupIP looks up host using the resolver and returns a slice of that host's IP addresses.
// It works like net.LookupIP but uses the recursive resolver instead of the system resolver.
// The function queries both A and AAAA records and returns all found IP addresses.
func (r *Resolver) LookupIP(host string) ([]net.IP, error) {
	var ips []net.IP

	// Query A records (IPv4)
	resultA, err := r.Resolve(host, dns.TypeA)
	if err == nil && resultA.Rcode == dns.RcodeSuccess {
		for _, rr := range resultA.Answer {
			if a, ok := rr.(*dns.A); ok {
				ips = append(ips, a.A)
			}
		}
	}

	// Query AAAA records (IPv6)
	resultAAAA, err := r.Resolve(host, dns.TypeAAAA)
	if err == nil && resultAAAA.Rcode == dns.RcodeSuccess {
		for _, rr := range resultAAAA.Answer {
			if aaaa, ok := rr.(*dns.AAAA); ok {
				ips = append(ips, aaaa.AAAA)
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP addresses found for %s", host)
	}

	return ips, nil
}

// LookupIPAll looks up host using the resolver and returns a slice of that host's IP addresses
// from ALL authoritative nameservers. It works like LookupIP but uses ResolveAll instead of Resolve,
// ensuring that IP addresses from all nameservers are collected and returned.
// The function queries both A and AAAA records in parallel and returns all found IP addresses.
func (r *Resolver) LookupIPAll(host string) ([]net.IP, error) {
	// Query A and AAAA records in parallel
	type resolveResult struct {
		result *Result
		err    error
	}
	
	aChan := make(chan resolveResult, 1)
	aaaaChan := make(chan resolveResult, 1)
	
	// Query A records (IPv4) from all nameservers
	go func() {
		result, err := r.ResolveAll(host, dns.TypeA)
		aChan <- resolveResult{result: result, err: err}
	}()
	
	// Query AAAA records (IPv6) from all nameservers
	go func() {
		result, err := r.ResolveAll(host, dns.TypeAAAA)
		aaaaChan <- resolveResult{result: result, err: err}
	}()
	
	var ips []net.IP
	
	// Collect A record results
	aResult := <-aChan
	if aResult.err == nil && aResult.result.Rcode == dns.RcodeSuccess {
		for _, rr := range aResult.result.Answer {
			if a, ok := rr.(*dns.A); ok {
				ips = append(ips, a.A)
			}
		}
	}
	
	// Collect AAAA record results
	aaaaResult := <-aaaaChan
	if aaaaResult.err == nil && aaaaResult.result.Rcode == dns.RcodeSuccess {
		for _, rr := range aaaaResult.result.Answer {
			if aaaa, ok := rr.(*dns.AAAA); ok {
				ips = append(ips, aaaa.AAAA)
			}
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP addresses found for %s", host)
	}

	return ips, nil
}

func (r *Resolver) makeCacheKey(domain string, qtype uint16) string {
	return domain + ":" + strconv.FormatUint(uint64(qtype), 10)
}

func (r *Resolver) calculateTTL(result *Result) time.Duration {
	if result == nil {
		return 0
	}

	minTTL := uint32(3600)

	for _, rr := range result.Answer {
		if rr.Header().Ttl < minTTL {
			minTTL = rr.Header().Ttl
		}
	}

	for _, rr := range result.Authority {
		if rr.Header().Ttl < minTTL {
			minTTL = rr.Header().Ttl
		}
	}

	if minTTL == 3600 && len(result.Answer) == 0 && len(result.Authority) == 0 {
		return 5 * time.Minute
	}

	if minTTL < 60 {
		minTTL = 60
	}

	return time.Duration(minTTL) * time.Second
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
		} else {
			// if missing glue, use local resolver to fix
			ips, err := net.LookupIP(nsName)
			if err == nil {
				for _, ip := range ips {
					nameservers = append(nameservers, net.JoinHostPort(ip.String(), "53"))
				}
			}
		}
	}

	return nameservers
}

// mergeResults combines multiple Result structs into a single Result,
// deduplicating records and using the best available Rcode and Authoritative status.
func mergeResults(results []*Result) *Result {
	if len(results) == 0 {
		return nil
	}

	merged := &Result{
		Answer:        []dns.RR{},
		Authority:     []dns.RR{},
		Additional:    []dns.RR{},
		Rcode:         dns.RcodeServerFailure,
		Authoritative: false,
	}

	rrSeen := make(map[string]bool)

	for _, result := range results {
		if result == nil {
			continue
		}

		if result.Rcode == dns.RcodeSuccess {
			merged.Rcode = dns.RcodeSuccess
		} else if merged.Rcode == dns.RcodeServerFailure && result.Rcode == dns.RcodeNameError {
			merged.Rcode = dns.RcodeNameError
		}

		if result.Authoritative {
			merged.Authoritative = true
		}

		for _, rr := range result.Answer {
			key := rr.String()
			if !rrSeen[key] {
				rrSeen[key] = true
				merged.Answer = append(merged.Answer, rr)
			}
		}

		for _, rr := range result.Authority {
			key := rr.String()
			if !rrSeen[key] {
				rrSeen[key] = true
				merged.Authority = append(merged.Authority, rr)
			}
		}

		for _, rr := range result.Additional {
			key := rr.String()
			if !rrSeen[key] {
				rrSeen[key] = true
				merged.Additional = append(merged.Additional, rr)
			}
		}
	}

	return merged
}

// resolveRecursiveAll performs recursive resolution by querying ALL nameservers
// at each level and merging their responses, unlike resolveRecursive which stops
// at the first successful response.
func (r *Resolver) resolveRecursiveAll(domain string, qtype uint16, nameservers []string, depth int) (*Result, error) {
	if depth > maxRecursionDepth {
		return nil, fmt.Errorf("maximum recursion depth exceeded")
	}

	if len(nameservers) == 0 {
		return nil, fmt.Errorf("no nameservers available")
	}

	// Query all nameservers in parallel
	type queryResult struct {
		result *Result
		err    error
	}
	
	resultChan := make(chan queryResult, len(nameservers))
	
	// Start goroutines for each nameserver
	for _, ns := range nameservers {
		go func(nameserver string) {
			result, err := r.queryNameserver(nameserver, domain, qtype)
			resultChan <- queryResult{result: result, err: err}
		}(ns)
	}
	
	// Collect results from all goroutines
	var allResults []*Result
	var hasSuccessfulQuery bool
	
	for i := 0; i < len(nameservers); i++ {
		qr := <-resultChan
		if qr.err != nil {
			continue
		}
		
		hasSuccessfulQuery = true
		allResults = append(allResults, qr.result)
	}

	if !hasSuccessfulQuery {
		return nil, fmt.Errorf("no nameservers responded for %s", domain)
	}

	merged := mergeResults(allResults)
	if merged == nil {
		return nil, fmt.Errorf("no valid responses for %s", domain)
	}

	if merged.Rcode != dns.RcodeSuccess {
		return merged, nil
	}

	if len(merged.Answer) > 0 {
		merged.Answer = r.followCNAME(merged.Answer, qtype, depth)
		return merged, nil
	}

	if len(merged.Authority) > 0 {
		nsRecords := r.extractNSRecords(merged.Authority)
		if len(nsRecords) == 0 {
			return merged, nil
		}

		nextNS := r.resolveNameservers(nsRecords, merged.Additional)
		if len(nextNS) == 0 {
			return merged, nil
		}

		return r.resolveRecursiveAll(domain, qtype, nextNS, depth+1)
	}

	return merged, nil
}
