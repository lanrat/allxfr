package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type zone struct {
	// map of names to nameservers
	ns map[string][]string
	// map of nameservers to ipv4 and ipv6
	ip map[string][]net.IP
}

/*type nsip struct {
	domain string
	ns     string
	ip     net.IP
}*/

func (z *zone) AddRecord(r dns.RR) {
	switch t := r.(type) {
	case *dns.A:
		z.AddIP(t.Hdr.Name, t.A)
	case *dns.AAAA:
		z.AddIP(t.Hdr.Name, t.AAAA)
	case *dns.NS:
		z.AddNS(t.Hdr.Name, t.Ns)
	}
}

func (z *zone) GetNameChan() chan string {
	out := make(chan string)
	go func() {
		for domain := range z.ns {
			// skip root & arpa
			if domain == "." {
				continue
			}
			parts := strings.Split(domain, ".")
			if parts[len(parts)-1] == "arpa" {
				continue
			}
			out <- domain
		}
		close(out)
	}()
	return out
}

func (z *zone) CountNS() int {
	return len(z.ns)
}

func (z *zone) AddNS(domain, nameserver string) {
	domain = strings.ToLower(domain)
	if z.ns == nil {
		z.ns = make(map[string][]string)
	}
	_, ok := z.ns[domain]
	if !ok {
		z.ns[domain] = make([]string, 0, 4)
	}
	if len(nameserver) > 0 {
		nameserver = strings.ToLower(nameserver)
		z.ns[domain] = append(z.ns[domain], nameserver)
	}
}

func (z *zone) AddIP(nameserver string, ip net.IP) {
	nameserver = strings.ToLower(nameserver)
	if z.ip == nil {
		z.ip = make(map[string][]net.IP)
	}
	_, ok := z.ip[nameserver]
	if !ok {
		z.ip[nameserver] = make([]net.IP, 0, 4)
	}
	z.ip[nameserver] = append(z.ip[nameserver], ip)
}

func (z *zone) Print() {
	fmt.Println("NS:")
	for zone := range z.ns {
		fmt.Printf("%s\n", zone)
		for i := range z.ns[zone] {
			fmt.Printf("\t%s\n", z.ns[zone][i])
		}
	}
	fmt.Println("IP:")
	for ns := range z.ip {
		fmt.Printf("%s\n", ns)
		for i := range z.ip[ns] {
			fmt.Printf("\t%s\n", z.ip[ns][i])
		}
	}
}

func (z *zone) PrintTree() {
	fmt.Println("Zones:")
	for zone := range z.ns {
		fmt.Printf("%s\n", zone)
		for i := range z.ns[zone] {
			fmt.Printf("\t%s\n", z.ns[zone][i])
			for j := range z.ip[z.ns[zone][i]] {
				fmt.Printf("\t\t%s\n", z.ip[z.ns[zone][i]][j])
			}
		}
	}
}
