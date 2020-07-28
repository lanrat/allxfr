package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"net"
	"os"
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
							log.Printf("[%s] %s", z, err)
						}
					} else {
						if records > 0 {
							break
						}
					}
				}
				if !*saveAll && records > 0 {
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
					log.Printf("[%s] %s", z, err)
				}
			} else {
				break
			}
		}

		for _, nameserver := range qNameservers {
			var qIPs []net.IP
			for try := 0; try < *retry; try++ {
				qIPs, err = queryIP(localNameserver, nameserver)
				if err != nil {
					if *verbose {
						log.Printf("[%s] %s", z, err)
					}
				} else {
					break
				}
			}

			for _, ip := range qIPs {
				ipString := string(ip.To16())
				if !ips[ipString] {
					ips[ipString] = true
					for try := 0; try < *retry; try++ {
						if *verbose {
							log.Printf("[%s] trying AXFR: %s %s %s", z, domain, nameserver, ip.String())
						}
						records, err = axfr(domain, nameserver, ip)
						if err != nil {
							if *verbose {
								log.Printf("[%s] %s", z, err)
							}
						} else {
							if records > 0 {
								break
							}
						}
					}
					if !*saveAll && records > 0 {
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
		log.Printf("%s %s (%s) xfr size: %d records in %s\n", domain, nameserver, ip.String(), records, took.String())
		atomic.AddUint32(&totalXFR, 1)
	}
	return records, err
}

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
		filename = fmt.Sprintf("%s/%s_%s_%s_zone.gz", *saveDir, zone, nameserver, ip.String())
	} else {
		filename = fmt.Sprintf("%s/%s.zone.gz", *saveDir, zone[:len(zone)-1])
	}
	if !*overwrite {
		if _, err := os.Stat(filename); err != nil && !os.IsNotExist(err) {
			if *verbose {
				log.Printf("[%s] file %q exists, skipping", zone, filename)
			}
			return 0, nil
		}
	}

	filenameTmp := fmt.Sprintf("%s.tmp", filename)
	var bufWriter *bufio.Writer

	var envelope, records int64
	for e := range env {
		if e.Error != nil {
			// skip on this error
			err = fmt.Errorf("transfer envelope error from zone: %s ip: %s (rec: %d, envelope: %d): %w", zone, ip.String(), records, envelope, e.Error)
			if *verbose {
				log.Printf("[%s] %s", zone, err)
			}
			err = nil
			break
		}
		if *dryRun && len(e.RR) > 0 {
			return int64(len(e.RR)), nil
		}
		for _, r := range e.RR {
			// create file here on first iteration of loop
			if bufWriter == nil {
				fileWriter, err := os.Create(filenameTmp)
				if err != nil {
					return records, err
				}
				gzWriter := gzip.NewWriter(fileWriter)
				gzWriter.ModTime = time.Now()
				gzWriter.Name = fmt.Sprintf("%s.zone", zone[:len(zone)-1])
				gzWriter.Comment = "generated by allxfr"
				bufWriter = bufio.NewWriter(gzWriter)
				// setup function to finish/close/safe the files when done
				defer func() {
					if records > 1 {
						// save record count comment at end of zone file
						err := writeComment(bufWriter, "records", fmt.Sprintf("%d", records))
						if err != nil {
							panic(err)
						}
						err = writeComment(bufWriter, "envelopes", fmt.Sprintf("%d", envelope))
						if err != nil {
							panic(err)
						}
					}
					err = bufWriter.Flush()
					if err != nil {
						panic(err)
					}
					err = gzWriter.Flush()
					if err != nil {
						panic(err)
					}
					err = gzWriter.Close()
					if err != nil {
						panic(err)
					}
					err = fileWriter.Close()
					if err != nil {
						panic(err)
					}
					if records > 1 {
						err = os.Rename(filenameTmp, filename)
					} else {
						err = os.Remove(filenameTmp)
					}
					if err != nil {
						panic(err)
					}
				}()
				// Save metadata to zone file as comment
				_, err = bufWriter.WriteString("; Generated by ALLXFR (https://github.com/lanrat/allxfr)\n")
				if err != nil {
					return records, err
				}
				err = writeComment(bufWriter, "timestamp", time.Now().Format(time.RFC3339))
				if err != nil {
					return records, err
				}
				err = writeComment(bufWriter, "zone", zone)
				if err != nil {
					return records, err
				}
				err = writeComment(bufWriter, "nameserver", nameserver)
				if err != nil {
					return records, err
				}
				err = writeComment(bufWriter, "nameserverIP", ip.String())
				if err != nil {
					return records, err
				}
				axfrType := "AXFR"
				if *ixfr {
					axfrType = "IXFR"
				}
				err = writeComment(bufWriter, "xfr", axfrType)
				if err != nil {
					return records, err
				}
			}
			_, err = bufWriter.WriteString(fmt.Sprintf("%s\n", RRString(r)))
			if err != nil {
				return records, err
			}
		}
		records += int64(len(e.RR))
		envelope++
	}

	return records, err
}
