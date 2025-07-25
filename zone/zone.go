package zone

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Zone contains the nameservers and nameserver IPs in a zone.
// It maintains mappings between domain names and their nameservers,
// as well as nameserver hostnames to their IP addresses.
type Zone struct {
	// NS maps domain names to their authoritative nameservers
	NS map[string][]string
	// IP maps nameserver hostnames to their IPv4 and IPv6 addresses
	IP map[string][]net.IP
	// Records tracks the total number of records added to the zone
	Records int64
}

// AddRecord adds NS, A, and AAAA records to the zone.
// It extracts nameserver and IP information from DNS resource records
// and updates the zone's internal mappings accordingly.
func (z *Zone) AddRecord(r dns.RR) {
	switch t := r.(type) {
	case *dns.A:
		z.AddIP(t.Hdr.Name, t.A)
	case *dns.AAAA:
		z.AddIP(t.Hdr.Name, t.AAAA)
	case *dns.NS:
		z.AddNS(t.Hdr.Name, t.Ns)
	}
}

// GetNameChan returns a channel that yields all domain names in the zone.
// It skips the root zone (".") and .arpa domains as they are not suitable
// for zone transfer attempts. The channel is closed when all domains are sent.
func (z *Zone) GetNameChan() chan string {
	out := make(chan string)
	go func() {
		for domain := range z.NS {
			// skip root & arpa
			if domain == "." {
				continue
			}
			if domain == "arpa." || strings.HasSuffix(domain, ".arpa.") {
				continue
			}
			out <- domain
		}
		close(out)
	}()
	return out
}

// CountNS returns the number of unique domain names that have nameservers in the zone.
func (z *Zone) CountNS() int {
	return len(z.NS)
}

// AddNS adds a nameserver for a domain to the zone.
// Domain names are normalized to lowercase. If nameserver is empty,
// only the domain entry is created without adding a nameserver.
func (z *Zone) AddNS(domain, nameserver string) {
	domain = strings.ToLower(domain)
	if z.NS == nil {
		z.NS = make(map[string][]string)
	}
	_, ok := z.NS[domain]
	if !ok {
		z.NS[domain] = make([]string, 0, 4)
	}
	if len(nameserver) > 0 {
		nameserver = strings.ToLower(nameserver)
		z.NS[domain] = append(z.NS[domain], nameserver)
		z.Records++
	}
}

// AddIP adds an IP address for a nameserver to the zone.
// Nameserver names are normalized to lowercase. Both IPv4 and IPv6
// addresses are supported and stored in the same slice.
func (z *Zone) AddIP(nameserver string, ip net.IP) {
	nameserver = strings.ToLower(nameserver)
	if z.IP == nil {
		z.IP = make(map[string][]net.IP)
	}
	_, ok := z.IP[nameserver]
	if !ok {
		z.IP[nameserver] = make([]net.IP, 0, 4)
	}
	z.IP[nameserver] = append(z.IP[nameserver], ip)
	z.Records++
}

// Print outputs the zone structure to stdout in a simple format.
// It displays all domains with their nameservers, followed by
// all nameservers with their IP addresses.
func (z *Zone) Print() {
	fmt.Println("NS:")
	for zone := range z.NS {
		fmt.Printf("%s\n", zone)
		for i := range z.NS[zone] {
			fmt.Printf("\t%s\n", z.NS[zone][i])
		}
	}
	fmt.Println("IP:")
	for ns := range z.IP {
		fmt.Printf("%s\n", ns)
		for i := range z.IP[ns] {
			fmt.Printf("\t%s\n", z.IP[ns][i])
		}
	}
}

// PrintTree outputs the zone structure to stdout in a hierarchical tree format.
// It shows domains, their nameservers, and the IP addresses for each nameserver
// in an indented tree structure for better readability.
func (z *Zone) PrintTree() {
	fmt.Println("Zones:")
	for zone := range z.NS {
		fmt.Printf("%s\n", zone)
		for i := range z.NS[zone] {
			fmt.Printf("\t%s\n", z.NS[zone][i])
			for j := range z.IP[z.NS[zone][i]] {
				fmt.Printf("\t\t%s\n", z.IP[z.NS[zone][i]][j])
			}
		}
	}
}
