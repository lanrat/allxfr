package main

import (
	"fmt"
	"log"
	"time"

	"github.com/miekg/dns"
)

func getRootServers() ([]string, error) {
	out := make([]string, 0, 4)
	// get root servers
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeNS)
	in, err := dns.Exchange(m, localNameserver)
	if err != nil {
		return out, err
	}
	for _, a := range in.Answer {
		if ns, ok := a.(*dns.NS); ok {
			out = append(out, ns.Ns)
		}
	}
	if len(out) == 0 {
		return out, fmt.Errorf("Unable to find Root Server")

	}
	return out, nil
}

func rootAXFR(ns string) (zone, error) {
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeAXFR)

	t := new(dns.Transfer)

	var root zone
	startTime := time.Now()

	env, err := t.In(m, fmt.Sprintf("%s:53", ns))
	if err != nil {
		return root, fmt.Errorf("transfer error from %v: %w", ns, err)
	}
	var envelope, record int
	for e := range env {
		if e.Error != nil {
			return root, fmt.Errorf("transfer envelope error from %v: %w", ns, e.Error)
		}
		for _, r := range e.RR {
			root.AddRecord(r)
		}
		record += len(e.RR)
		envelope++
	}
	took := time.Since(startTime).Round(time.Millisecond)
	log.Printf("ROOT %s xfr size: %d records in %s \n", ns, record, took.String())

	return root, nil
}
