package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/miekg/dns"
)

// axfrWorker iterate through all possabilities and queries attempting an AXFR
func axfrWorker(z zone, domain string) error {
	ips := make(map[string]bool)
	domain = dns.Fqdn(domain)
	filename := fmt.Sprintf("%s/%s.zone.gz", *saveDir, domain[:len(domain)-1])
	for _, nameserver := range z.ns[domain] {
		for _, ip := range z.ip[nameserver] {
			ipString := string(ip.To16())
			if !ips[ipString] {
				ips[ipString] = true
				if *saveAll {
					filename = fmt.Sprintf("%s/%s_%s_%s_zone.gz", *saveDir, domain, nameserver, ip.String())
				}
				records, err := axfr(domain, nameserver, ip, filename)
				if err != nil {
					return err
				}
				if !*saveAll && records > 0 {
					return nil
				}
			}
		}
	}
	if len(*ns) > 0 {
		// query NS and run axfr on missing IPs
		qNameservers, err := queryNS(localNameserver, domain)
		if err != nil {
			if *verbose {
				log.Println(err)
			}
			return nil
		}
		for _, nameserver := range qNameservers {
			qIPs, err := queryIP(localNameserver, nameserver)
			if err != nil {
				if *verbose {
					log.Println(err)
				}
				continue
			}

			for _, ip := range qIPs {
				ipString := string(ip.To16())
				if !ips[ipString] {
					ips[ipString] = true
					if *saveAll {
						filename = fmt.Sprintf("%s/%s_%s_%s_zone.gz", *saveDir, domain, nameserver, ip.String())
					}
					if *verbose {
						log.Printf("Trying AXFR: %s %s %s", domain, nameserver, ip.String())
					}
					records, err := axfr(domain, nameserver, ip, filename)
					if err != nil {
						return err
					}
					if !*saveAll && records > 0 {
						return nil
					}
				}
			}
		}
	}
	return nil
}

func axfr(domain, nameserver string, ip net.IP, filename string) (int64, error) {
	startTime := time.Now()
	records, err := axfrToFile(domain, ip, filename)
	if err == nil && records > 0 {
		took := time.Since(startTime).Round(time.Second / 1000)
		log.Printf("%s %s (%s) xfr size: %d records in %s\n", domain, nameserver, ip.String(), records, took.String())

	}
	return records, err
}

func axfrToFile(zone string, ip net.IP, filename string) (int64, error) {
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
			log.Print(err)
		}
		return 0, nil
	}

	// get ready to save file
	filenameTmp := fmt.Sprintf("%s.tmp", filename)
	var bufWriter *bufio.Writer

	var envelope, record int64
	for e := range env {
		if e.Error != nil {
			// skip on this error
			err = fmt.Errorf("transfer envelope error from zone: %s ip: %s (rec: %d, envelope: %d): %w", zone, ip.String(), record, envelope, e.Error)
			if *verbose {
				log.Print(err)
			}
			err = nil
			break
		}
		for _, r := range e.RR {
			// create file here on first iteration of loop
			if bufWriter == nil {
				fileWriter, err := os.Create(filenameTmp)
				if err != nil {
					return record, err
				}
				gzWriter := gzip.NewWriter(fileWriter)
				bufWriter = bufio.NewWriter(gzWriter)
				defer func() {
					bufWriter.Flush()
					gzWriter.Flush()
					gzWriter.Close()
					fileWriter.Close()
					if record > 1 {
						os.Rename(filenameTmp, filename)
					} else {
						os.Remove(filenameTmp)
					}
				}()
			}
			_, err = bufWriter.WriteString(fmt.Sprintf("%s\n", r.String()))
			if err != nil {
				return record, err
			}
		}
		record += int64(len(e.RR))
		envelope++
	}

	return record, err
}
