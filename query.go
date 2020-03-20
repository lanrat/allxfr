package main

import (
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

var client dns.Client

func init() {
	client.Timeout = globalTimeout
	client.Dialer = &net.Dialer{
		Timeout: globalTimeout,
	}
}

// NOTE: these query functions are not fully recursive
// they are meant to be used with a fully recursive resolver like unbound/bind/named

func queryNS(server, domain string) ([]string, error) {
	domain = dns.Fqdn(domain)
	if *verbose {
		log.Printf("DNS Query: @%s NS %s", server, domain)
	}
	m := new(dns.Msg)
	m.SetQuestion(domain, dns.TypeNS)

	in, _, err := client.Exchange(m, server)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, 2)
	for i := range in.Answer {
		if t, ok := in.Answer[i].(*dns.NS); ok {
			if *verbose {
				log.Printf("\t%s\n", t.Ns)
			}
			t.Ns = strings.ToLower(t.Ns)
			out = append(out, t.Ns)
		}
	}

	return out, nil
}

func queryA(server, domain string) ([]net.IP, error) {
	domain = dns.Fqdn(domain)
	if *verbose {
		log.Printf("DNS Query: @%s A %s", server, domain)
	}
	m := new(dns.Msg)
	m.SetQuestion(domain, dns.TypeA)

	in, _, err := client.Exchange(m, server)
	if err != nil {
		return nil, err
	}

	out := make([]net.IP, 0, 1)
	for i := range in.Answer {
		if t, ok := in.Answer[i].(*dns.A); ok {
			if *verbose {
				log.Printf("\t%s\n", t.A)
			}
			out = append(out, t.A)
		}
	}

	return out, nil
}

func queryAAAA(server, domain string) ([]net.IP, error) {
	domain = dns.Fqdn(domain)
	if *verbose {
		log.Printf("DNS Query: @%s AAAA %s", server, domain)
	}
	m := new(dns.Msg)
	m.SetQuestion(domain, dns.TypeAAAA)

	in, _, err := client.Exchange(m, server)
	if err != nil {
		return nil, err
	}

	out := make([]net.IP, 0, 1)
	for i := range in.Answer {
		if t, ok := in.Answer[i].(*dns.AAAA); ok {
			if *verbose {
				log.Printf("\t%s\n", t.AAAA)
			}
			out = append(out, t.AAAA)
		}
	}

	return out, nil
}

func queryIP(server, domain string) ([]net.IP, error) {
	aips, err := queryA(server, domain)
	if err != nil {
		return aips, err
	}
	aaaaips, err := queryAAAA(server, domain)
	return append(aips, aaaaips...), err
}
