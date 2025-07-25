package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/lanrat/allxfr/save"
	"github.com/lanrat/allxfr/zone"

	"github.com/miekg/dns"
)

var ErrAxfrUnsupported = errors.New("AXFR Unsupported")

func ErrorAxfrUnsupportedWrap(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	// "bad xfr rcode: 5" - Refused
	if errStr == "dns: bad xfr rcode: 5" {
		err = fmt.Errorf("%w 'Refused': %w", ErrAxfrUnsupported, err)
	}
	//"bad xfr rcode: 9" - Not Authorized / Not Authenticated
	if errStr == "dns: bad xfr rcode: 9" {
		err = fmt.Errorf("%w 'Not Authorized': %w", ErrAxfrUnsupported, err)
	}
	return err
}

// axfrWorker iterate through all possibilities and queries attempting an AXFR
func axfrWorker(z zone.Zone, domain string) error {
	attemptedIPs := make(map[string]bool)
	domain = dns.Fqdn(domain)
	var err error
	//var records int64
	var anySuccess bool
	for _, nameserver := range z.NS[domain] {
		for _, ip := range z.IP[nameserver] {
			ipString := ip.To16().String()
			if !attemptedIPs[ipString] {
				attemptedIPs[ipString] = true
				anySuccess, err = axfrRetry(ip, domain, nameserver)
				if err != nil {
					continue
				}
				if anySuccess {
					// got the zone
					return nil
				}
			}
		}
	}

	// query NS and run axfr on missing IPs
	var qNameservers []string
	for try := 0; try < *retry; try++ {
		result, err := resolve.Resolve(domain, dns.TypeNS)
		if err != nil {
			v("[%s] %s", domain, err)
		} else {
			for _, rr := range result.Answer {
				if ns, ok := rr.(*dns.NS); ok {
					qNameservers = append(qNameservers, ns.Ns)
				}
			}
			break
		}
		time.Sleep(1 * time.Second)
	}

	for _, nameserver := range qNameservers {
		var qIPs []net.IP
		for try := 0; try < *retry; try++ {
			qIPs, err = resolve.LookupIPAll(nameserver)
			if err != nil {
				v("[%s] %s", domain, err)
			} else {
				break
			}
			time.Sleep(1 * time.Second)
		}

		for _, ip := range qIPs {
			ipString := ip.To16().String()
			if !attemptedIPs[ipString] {
				attemptedIPs[ipString] = true
				v("[%s] trying non-glue AXFR: %s %s", domain, nameserver, ip.String())
				anySuccess, err = axfrRetry(ip, domain, nameserver)
				if err != nil {
					continue
				}
				if anySuccess {
					// got the zone
					return nil
				}
			}
		}
	}

	// If no successful transfers occurred, mark domain as failed
	if !anySuccess && statusServer != nil {
		statusServer.FailTransfer(domain, "no successful zone transfers")
	}

	return nil
}

func axfrRetry(ip net.IP, domain, nameserver string) (bool, error) {
	var err error
	var records int64
	var anySuccess bool

	for try := 0; try < *retry; try++ {
		records, err = axfr(domain, nameserver, ip)
		if err != nil {
			v("[%s] %s", domain, err)
			// if axfr is unsupported by NS, then move on, otherwise retry
			if errors.Is(err, ErrAxfrUnsupported) {
				err = nil
				// skip remaining tries with this IP
				break
			}
		} else {
			if records != 0 {
				anySuccess = true
				break
			}
		}
		time.Sleep(1 * time.Second)
	}
	if !*saveAll && records != 0 {
		return anySuccess, nil
	}
	if err != nil {
		return anySuccess, err
	}

	return anySuccess, err
}

func axfr(domain, nameserver string, ip net.IP) (int64, error) {
	startTime := time.Now()
	records, err := axfrToFile(domain, ip, nameserver)
	if err == nil && records > 0 {
		took := time.Since(startTime).Round(time.Millisecond)
		log.Printf("[%s] %s (%s) xfr size: %d records in %s\n", domain, nameserver, ip.String(), records, took.String())
		atomic.AddUint32(&totalXFR, 1)

		// Update status server on successful transfer
		if statusServer != nil {
			statusServer.CompleteTransfer(domain)
		}
	}
	return records, err
}

// returns -1 if zone already exists and we are not overwriting
func axfrToFile(zone string, ip net.IP, nameserver string) (int64, error) {
	zone = dns.Fqdn(zone)

	m := new(dns.Msg)
	if *ixfr {
		m.SetIxfr(zone, 0, "", "")
	} else {
		m.SetQuestion(zone, dns.TypeAXFR)
	}

	t := new(dns.Transfer)
	t.DialTimeout = globalTimeout
	t.ReadTimeout = globalTimeout
	t.WriteTimeout = globalTimeout
	env, err := t.In(m, net.JoinHostPort(ip.String(), "53"))
	if err != nil {
		// skip on this error
		err = fmt.Errorf("transfer error from zone: %s ip: %s: %w", zone, ip.String(), err)
		v("[%s] %s", zone, err)
		return 0, nil
	}

	// get ready to save file
	var filename string
	if *saveAll {
		filename = path.Join(*saveDir, fmt.Sprintf("%s_%s_%s_zone.gz", zone, nameserver, ip.String()))
	} else {
		filename = path.Join(*saveDir, fmt.Sprintf("%s.zone.gz", zone[:len(zone)-1]))
	}
	if !*overwrite {
		if _, err := os.Stat(filename); err == nil || !os.IsNotExist(err) {
			v("[%s] file %q exists, skipping", zone, filename)
			return -1, nil
		}
	}

	var envelope int64
	zonefile := save.New(zone, filename)
	defer func() {
		err = zonefile.WriteCommentKey("envelopes", fmt.Sprintf("%d", envelope))
		if err != nil {
			panic(err)
		}
		err := zonefile.Finish()
		if err != nil {
			panic(err)
		}
	}()
	err = zonefile.WriteComment("Generated by ALLXFR (https://github.com/lanrat/allxfr)\n")
	if err != nil {
		return zonefile.Records(), err
	}
	err = zonefile.WriteCommentKey("nameserver", nameserver)
	if err != nil {
		return zonefile.Records(), err
	}
	err = zonefile.WriteCommentKey("nameserverIP", ip.String())
	if err != nil {
		return zonefile.Records(), err
	}
	axfrType := "AXFR"
	if *ixfr {
		axfrType = "IXFR"
	}
	err = zonefile.WriteCommentKey("xfr", axfrType)
	if err != nil {
		return zonefile.Records(), err
	}

	for e := range env {
		if e.Error != nil {
			err = ErrorAxfrUnsupportedWrap(e.Error)
			// skip on this error
			err = fmt.Errorf("transfer envelope error from zone: %s ip: %s (rec: %d, envelope: %d): %w", zone, ip.String(), zonefile.Records(), envelope, err)
			return zonefile.Records(), err
		}
		// zonefile will not write anything to disk unless it has been provided records to write.
		if *dryRun && len(e.RR) > 0 {
			return int64(len(e.RR)), nil
		}
		for _, rr := range e.RR {
			// create file here on first iteration of loop
			err := zonefile.AddRR(rr)
			if err != nil {
				return zonefile.Records(), err
			}
		}
		envelope++
	}

	return zonefile.Records(), err
}
