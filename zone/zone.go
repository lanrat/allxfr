package zone

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Zone contains the nameservers and nameserver IPs in a zone
type Zone struct {
	// map of names to nameservers
	NS map[string][]string
	// map of nameservers to ipv4 and ipv6
	IP map[string][]net.IP
	// number of records added to the zone
	Records int64
}

// AddRecord adds NS, A, AAAA records to the zone
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

// GetNameChan returns a channel of domains in the zone
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

// CountNS returns the number of nameservers in the zone
func (z *Zone) CountNS() int {
	return len(z.NS)
}

// AddNS adds a domain nameserver pair to the zone
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

// AddIP adds a nameserver IP pair to the zone
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

// Print prints the zone to stdout
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

// PrintTree prints the zone in tree format to stdout
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
