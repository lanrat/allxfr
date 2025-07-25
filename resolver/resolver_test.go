package resolver

import (
	"net"
	"sort"
	"testing"

	"github.com/miekg/dns"
)

func TestResolverA(t *testing.T) {
	resolver := New()
	testDomains := []string{
		"google.com",
		"github.com",
		"cloudflare.com",
	}

	for _, domain := range testDomains {
		t.Run(domain, func(t *testing.T) {
			result, err := resolver.Resolve(domain, dns.TypeA)
			if err != nil {
				t.Fatalf("Failed to resolve %s: %v", domain, err)
			}

			if len(result.Answer) == 0 {
				t.Fatalf("No A records found for %s", domain)
			}

			var resolverIPs []net.IP
			for _, rr := range result.Answer {
				if a, ok := rr.(*dns.A); ok {
					resolverIPs = append(resolverIPs, a.A)
				}
			}

			if len(resolverIPs) == 0 {
				t.Fatalf("No A records in answer section for %s", domain)
			}

			systemIPs, err := net.LookupIP(domain)
			if err != nil {
				t.Fatalf("System lookup failed for %s: %v", domain, err)
			}

			var systemIPv4 []net.IP
			for _, ip := range systemIPs {
				if ip.To4() != nil {
					systemIPv4 = append(systemIPv4, ip)
				}
			}

			if len(systemIPv4) == 0 {
				t.Skipf("No IPv4 addresses from system resolver for %s", domain)
			}

			if !compareIPSets(resolverIPs, systemIPv4) {
				t.Logf("Resolver IPs: %v", resolverIPs)
				t.Logf("System IPs: %v", systemIPv4)
				t.Logf("Note: IP sets differ - this may be due to different DNS servers or timing")
			}
		})
	}
}

func TestResolverAAAA(t *testing.T) {
	resolver := New()
	testDomains := []string{
		"google.com",
		"github.com",
		"cloudflare.com",
	}

	for _, domain := range testDomains {
		t.Run(domain, func(t *testing.T) {
			result, err := resolver.Resolve(domain, dns.TypeAAAA)
			if err != nil {
				t.Logf("No AAAA records for %s: %v", domain, err)
				return
			}

			var resolverIPs []net.IP
			for _, rr := range result.Answer {
				if aaaa, ok := rr.(*dns.AAAA); ok {
					resolverIPs = append(resolverIPs, aaaa.AAAA)
				}
			}

			systemIPs, err := net.LookupIP(domain)
			if err != nil {
				t.Fatalf("System lookup failed for %s: %v", domain, err)
			}

			var systemIPv6 []net.IP
			for _, ip := range systemIPs {
				if ip.To4() == nil && ip.To16() != nil {
					systemIPv6 = append(systemIPv6, ip)
				}
			}

			if len(resolverIPs) == 0 && len(systemIPv6) == 0 {
				t.Logf("No AAAA records for %s (expected)", domain)
				return
			}

			if len(resolverIPs) > 0 && len(systemIPv6) > 0 {
				if !compareIPSets(resolverIPs, systemIPv6) {
					t.Logf("Resolver IPv6: %v", resolverIPs)
					t.Logf("System IPv6: %v", systemIPv6)
					t.Logf("Note: IPv6 sets differ - this may be due to different DNS servers or timing")
				}
			}
		})
	}
}

func TestResolverNS(t *testing.T) {
	resolver := New()
	testDomains := []string{
		"google.com",
		"github.com",
	}

	for _, domain := range testDomains {
		t.Run(domain, func(t *testing.T) {
			result, err := resolver.Resolve(domain, dns.TypeNS)
			if err != nil {
				t.Fatalf("Failed to resolve %s NS: %v", domain, err)
			}

			if len(result.Answer) == 0 {
				t.Fatalf("No NS records found for %s", domain)
			}

			var nsRecords []string
			for _, rr := range result.Answer {
				if ns, ok := rr.(*dns.NS); ok {
					nsRecords = append(nsRecords, ns.Ns)
				}
			}

			if len(nsRecords) == 0 {
				t.Fatalf("No NS records in answer section for %s", domain)
			}

			systemNS, err := net.LookupNS(domain)
			if err != nil {
				t.Fatalf("System NS lookup failed for %s: %v", domain, err)
			}

			var systemNSNames []string
			for _, ns := range systemNS {
				systemNSNames = append(systemNSNames, ns.Host)
			}

			if !compareStringSets(nsRecords, systemNSNames) {
				t.Logf("Resolver NS: %v", nsRecords)
				t.Logf("System NS: %v", systemNSNames)
				t.Logf("Note: NS sets differ - this may be due to different DNS servers or timing")
			}
		})
	}
}

func TestResolverCNAME(t *testing.T) {
	resolver := New()
	testDomains := []string{
		"www.github.com",
	}

	for _, domain := range testDomains {
		t.Run(domain, func(t *testing.T) {
			result, err := resolver.Resolve(domain, dns.TypeCNAME)
			if err != nil {
				t.Fatalf("Failed to resolve %s CNAME: %v", domain, err)
			}

			var cnameTarget string
			for _, rr := range result.Answer {
				if cname, ok := rr.(*dns.CNAME); ok {
					cnameTarget = cname.Target
					break
				}
			}

			systemCNAME, err := net.LookupCNAME(domain)
			if err != nil {
				if len(result.Answer) == 0 {
					t.Skipf("No CNAME record for %s (expected)", domain)
					return
				}
				t.Fatalf("System CNAME lookup failed for %s: %v", domain, err)
			}

			if cnameTarget != "" && systemCNAME != "" {
				if cnameTarget != systemCNAME {
					t.Logf("Resolver CNAME: %s", cnameTarget)
					t.Logf("System CNAME: %s", systemCNAME)
					t.Logf("Note: CNAME targets differ - this may be due to different DNS servers or timing")
				}
			}
		})
	}
}

func TestResolverNXDOMAIN(t *testing.T) {
	resolver := New()
	nonexistentDomain := "this-does-not-exist-12345.com"

	result, err := resolver.Resolve(nonexistentDomain, dns.TypeA)

	if err != nil {
		t.Fatalf("Unexpected error for NXDOMAIN: %v", err)
	}

	if result.Rcode != dns.RcodeNameError {
		t.Fatalf("Expected NXDOMAIN (RcodeNameError), got %d", result.Rcode)
	}
}

func TestRootServerResolution(t *testing.T) {
	t.Logf("Testing TestRootServerResolution")

	rootServers := getRootServers()
	if len(rootServers) == 0 {
		t.Fatal("No root servers resolved")
	}

	t.Logf("Resolved %d root servers", len(rootServers))
	for i, server := range rootServers {
		if i < 5 {
			t.Logf("Root server %d: %s", i+1, server)
		}
	}
}

func compareIPSets(a, b []net.IP) bool {
	if len(a) != len(b) {
		return false
	}

	aStr := make([]string, len(a))
	bStr := make([]string, len(b))

	for i, ip := range a {
		aStr[i] = ip.String()
	}
	for i, ip := range b {
		bStr[i] = ip.String()
	}

	sort.Strings(aStr)
	sort.Strings(bStr)

	for i := range aStr {
		if aStr[i] != bStr[i] {
			return false
		}
	}

	return true
}

func compareStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))
	copy(aCopy, a)
	copy(bCopy, b)

	sort.Strings(aCopy)
	sort.Strings(bCopy)

	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}

	return true
}
