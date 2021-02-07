package main

import (
	"allxfr/save"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// axfrWorker iterate through all possabilities and queries attempting an AXFR
func axfrWorker(z zone, domain string) error {
	ips := make(map[string]bool)
	domain = dns.Fqdn(domain)
	var err error
	var records int64
	for _, nameserver := range z.ns[domain] {
		for _, ip := range z.ip[nameserver] {
			ipString := string(ip.To16())
			if !ips[ipString] {
				ips[ipString] = true
				for try := 0; try < *retry; try++ {
					records, err = axfr(domain, nameserver, ip)
					if err != nil {
						if *verbose {
							log.Printf("[%s] %s", domain, err)
						}
					} else {
						if records != 0 {
							break
						}
					}
					time.Sleep(1 * time.Second)
				}
				if !*saveAll && records != 0 {
					return nil
				}
				if err != nil {
					return err
				}
			}
		}
	}
	if len(*ns) > 0 {
		// query NS and run axfr on missing IPs
		var qNameservers []string
		for try := 0; try < *retry; try++ {
			qNameservers, err = queryNS(localNameserver, domain)
			if err != nil {
				if *verbose {
					log.Printf("[%s] %s", domain, err)
				}
			} else {
				break
			}
			time.Sleep(1 * time.Second)
		}

		for _, nameserver := range qNameservers {
			var qIPs []net.IP
			for try := 0; try < *retry; try++ {
				qIPs, err = queryIP(localNameserver, nameserver)
				if err != nil {
					if *verbose {
						log.Printf("[%s] %s", domain, err)
					}
				} else {
					break
				}
				time.Sleep(1 * time.Second)
			}

			for _, ip := range qIPs {
				ipString := string(ip.To16())
				if !ips[ipString] {
					ips[ipString] = true
					for try := 0; try < *retry; try++ {
						if *verbose {
							log.Printf("[%s] trying AXFR: %s %s", domain, nameserver, ip.String())
						}
						records, err = axfr(domain, nameserver, ip)
						if err != nil {
							if *verbose {
								log.Printf("[%s] %s", domain, err)
							}
						} else {
							if records != 0 {
								break
							}
						}
						time.Sleep(1 * time.Second)
					}
					if !*saveAll && records != 0 {
						return nil
					}
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func axfr(domain, nameserver string, ip net.IP) (int64, error) {
	startTime := time.Now()
	records, err := axfrToFile(domain, ip, nameserver)
	if err == nil && records > 0 {
		took := time.Since(startTime).Round(time.Millisecond)
		log.Printf("[%s] %s (%s) xfr size: %d records in %s\n", domain, nameserver, ip.String(), records, took.String())
		atomic.AddUint32(&totalXFR, 1)
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
		if *verbose {
			log.Printf("[%s] %s", zone, err)
		}
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
			if *verbose {
				log.Printf("[%s] file %q exists, skipping", zone, filename)
			}
			return -1, nil
		}
	}

	var envelope int64
	if *verbose {
		log.Printf("saving zone %q to file %s", zone, filename)
	}
	zonefile := save.New(zone, filename)
	defer func() {
		err = zonefile.WriteComment("envelopes", fmt.Sprintf("%d", envelope))
		if err != nil {
			panic(err)
		}
		err := zonefile.Finish()
		if err != nil {
			panic(err)
		}
	}()

	err = zonefile.WriteComment("nameserver", nameserver)
	if err != nil {
		return zonefile.Records(), err
	}
	err = zonefile.WriteComment("nameserverIP", ip.String())
	if err != nil {
		return zonefile.Records(), err
	}
	axfrType := "AXFR"
	if *ixfr {
		axfrType = "IXFR"
	}
	err = zonefile.WriteComment("xfr", axfrType)
	if err != nil {
		return zonefile.Records(), err
	}

	for e := range env {
		if e.Error != nil {
			// skip on this error
			err = fmt.Errorf("transfer envelope error from zone: %s ip: %s (rec: %d, envelope: %d): %w", zone, ip.String(), zonefile.Records(), envelope, e.Error)
			if *verbose {
				log.Printf("[%s] %s", zone, err)
			}
			err = nil
			break
		}
		// TODO don't need to create empty zone files for dry runs
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
