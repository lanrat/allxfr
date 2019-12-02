package main

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
)

type zone struct {
	// map of names to nameservers
	ns map[string][]string
	// map of nameservers to ipv4 and ipv6
	ip map[string][]net.IP
}

type nsip struct {
	domain string
	ns     string
	ip     net.IP
}

func (z *zone) AddRecord(r dns.RR) {
	switch t := r.(type) {
	case *dns.A:
		z.AddIP(t.Hdr.Name, t.A)
	case *dns.AAAA:
		z.AddIP(t.Hdr.Name, t.AAAA)
	case *dns.NS:
		z.AddTLD(t.Hdr.Name, t.Ns)
	}
}

func (z *zone) GetNsIPChan() chan nsip {
	out := make(chan nsip)
	go func() {
		for domain := range z.ns {
			// skip root & arpa
			if domain == "." {
				continue
			}
			if domain == "arpa." {
				continue
			}
			for _, ns := range z.ns[domain] {
				for _, ip := range z.ip[ns] {
					out <- nsip{domain: domain, ns: ns, ip: ip}
				}
			}
		}
		close(out)
	}()
	return out
}

func (z *zone) CountNS() int {
	return len(z.ns)
}

func (z *zone) AddTLD(tld, nameserver string) {
	if z.ns == nil {
		z.ns = make(map[string][]string)
	}
	_, ok := z.ns[tld]
	if !ok {
		z.ns[tld] = make([]string, 0, 4)
	}
	z.ns[tld] = append(z.ns[tld], nameserver)
}

func (z *zone) AddIP(nameserver string, ip net.IP) {
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
