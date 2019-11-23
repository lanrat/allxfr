package main

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

type root struct {
	// map of TLDs to nameservers
	tld map[string][]string
	// map of nameservers to ipv4 and ipv6
	ip map[string][]net.IP
}

func (r *root) AddTLD(tld, nameserver string) {
	if r.tld == nil {
		r.tld = make(map[string][]string)
	}
	_, ok := r.tld[tld]
	if !ok {
		r.tld[tld] = make([]string, 0, 4)
	}
	r.tld[tld] = append(r.tld[tld], nameserver)
}

func (r *root) AddIP(nameserver string, ip net.IP) {
	if r.ip == nil {
		r.ip = make(map[string][]net.IP)
	}
	_, ok := r.ip[nameserver]
	if !ok {
		r.ip[nameserver] = make([]net.IP, 0, 4)
	}
	r.ip[nameserver] = append(r.ip[nameserver], ip)
}

func (r *root) Print() {
	fmt.Println("NS:")
	for zone := range r.tld {
		fmt.Printf("%s\n", zone)
		for i := range r.tld[zone] {
			fmt.Printf("\t%s\n", r.tld[zone][i])
		}
	}
	fmt.Println("IP:")
	for ns := range r.ip {
		fmt.Printf("%s\n", ns)
		for i := range r.ip[ns] {
			fmt.Printf("\t%s\n", r.ip[ns][i])
		}
	}
}

func (r *root) PrintTree() {
	fmt.Println("Zones:")
	for zone := range r.tld {
		fmt.Printf("%s\n", zone)
		for i := range r.tld[zone] {
			fmt.Printf("\t%s\n", r.tld[zone][i])
			for j := range r.ip[r.tld[zone][i]] {
				fmt.Printf("\t\t%s\n", r.ip[r.tld[zone][i]][j])
			}
		}
	}
}

func main() {
	server, err := getRootServer()
	check(err)

	log.Printf("using server %s", server)
	err = rootAXFR(server)
	check(err)
	log.Printf("done")
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func rootAXFR(ns string) error {
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeAXFR)

	t := new(dns.Transfer)

	var root root

	env, err := t.In(m, fmt.Sprintf("%s:53", ns))
	if err != nil {
		return err
	}
	var envelope, record int
	for e := range env {
		//fmt.Println("envelope loop") // 108, contains abut 200 records
		if e.Error != nil {
			return e.Error
		}
		for _, r := range e.RR {
			//fmt.Println("RR loop") // 22077
			//fmt.Printf("IAN: %s\n", r)
			switch t := r.(type) {
			case *dns.A:
				root.AddIP(t.Hdr.Name, t.A)
			case *dns.AAAA:
				root.AddIP(t.Hdr.Name, t.AAAA)
			case *dns.NS:
				root.AddTLD(t.Hdr.Name, t.Ns)
			}
		}
		record += len(e.RR)
		envelope++
	}
	log.Printf("\n;; xfr size: %d records (envelopes %d)\n", record, envelope)

	root.PrintTree()
	return nil
}

var ErrNoRoot = fmt.Errorf("Unable to find Root Server")

func getRootServer() (string, error) {
	// get root server from local DNS
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return "", err
	}
	localserver := fmt.Sprintf("%s:%s", conf.Servers[0], conf.Port)

	// get root servers
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeNS)
	in, err := dns.Exchange(m, localserver)
	if err != nil {
		return "", err
	}
	//fmt.Println(in)
	for _, a := range in.Answer {
		if ns, ok := a.(*dns.NS); ok {
			return ns.Ns, nil
		}
	}
	return "", ErrNoRoot
}
