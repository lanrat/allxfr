package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/miekg/dns"
)

func axfr(zone, nameserver string, ip net.IP) error {
	if *verbose {
		log.Printf("Trying AXFR: %s %s %s", zone, nameserver, ip.String())
	}
	m := new(dns.Msg)
	m.SetQuestion(zone, dns.TypeAXFR)

	ipString := ip.String()
	if ip.To4() == nil {
		ipString = fmt.Sprintf("[%s]", ipString)
	}

	t := new(dns.Transfer)
	env, err := t.In(m, fmt.Sprintf("%s:53", ipString))
	if err != nil {
		// skip on this error
		err = fmt.Errorf("transfer error from %s (%s): %w", nameserver, ip.String(), err)
		if *verbose {
			log.Print(err)
		}
		return nil
	}

	// get ready to save file
	filename := fmt.Sprintf("%s/%s_%s_%s.zone.gz", *saveDir, zone, nameserver, ip.String())
	filenameTmp := fmt.Sprintf("%s.tmp", filename)
	fi, err := os.Create(filenameTmp)
	if err != nil {
		return err
	}
	gf := gzip.NewWriter(fi)
	fw := bufio.NewWriter(gf)

	var envelope, record int
	for e := range env {
		if e.Error != nil {
			// skip on this error
			err = fmt.Errorf("transfer envelope error from %v: %w", nameserver, e.Error)
			if *verbose {
				log.Print(err)
			}
			break
		}
		for _, r := range e.RR {
			//fmt.Printf("%s\n", r)
			_, err = fw.WriteString(fmt.Sprintf("%s\n", r.String()))
			if err != nil {
				fw.Flush()
				gf.Close()
				fi.Close()
				os.Remove(filenameTmp)
				return err
			}

		}
		record += len(e.RR)
		envelope++
	}
	fw.Flush()
	gf.Close()
	fi.Close()
	if record > 1 {
		log.Printf("%s %s (%s) xfr size: %d records (envelopes %d)\n", zone, nameserver, ip.String(), record, envelope)
		err = os.Rename(filenameTmp, filename)
	} else {
		err = os.Remove(filenameTmp)
	}
	if err != nil {
		return err
	}

	return nil
}