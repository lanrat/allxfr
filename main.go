package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

var (
	nameserver = flag.String("ns", "", "nameserver to use to get the root, if not set system default is used")
	parallel   = flag.Uint("parallel", 10, "number of parallel zone transfers to perform")
	saveDir    = flag.String("out", ".", "directory to save found zones in")
	verbose    = flag.Bool("verbose", false, "enable verbose output")
	zonefile   = flag.String("zonefile", "", "use the provided zonefile instead of getting the root zonefile")
)

func main() {
	flag.Parse()
	localNameserver, err := getNameserver()
	check(err)
	if *verbose {
		log.Printf("Using initial nameserver %s", localNameserver)
	}
	rootNameservers, err := getRootServers(localNameserver)
	check(err)

	var z zone
	if len(*zonefile) == 0 {
		// get zone file from root AXFR
		// not all the root nameservers allow AXFR, try them until we find one that does
		for _, ns := range rootNameservers {
			if *verbose {
				log.Printf("Trying root nameserver %s", ns)
			}
			z, err = rootAXFR(ns)
			if err == nil {
				break
			}
		}
	} else {
		// zone file is provided
		z, err = parseZoneFile(*zonefile)
		check(err)

	}

	if z.CountNS() == 0 {
		log.Fatal("Got empty zone")
	}

	// create outpout dir if does not exist
	if _, err := os.Stat(*saveDir); os.IsNotExist(err) {
		err = os.MkdirAll(*saveDir, os.ModePerm)
		check(err)
	}

	// TODO print size and keep track of progress

	if *verbose {
		z.PrintTree()
	}
	rootChan := z.GetNsIPChan()
	var g errgroup.Group

	// start workers
	for i := uint(0); i < *parallel; i++ {
		g.Go(func() error { return worker(rootChan) })
	}

	err = g.Wait()
	check(err)
}

func worker(c chan nsip) error {
	for {
		r, more := <-c
		if !more {
			return nil
		}
		err := axfr(r.domain, r.ns, r.ip)
		if err != nil {
			return err
		}
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getNameserver() (string, error) {
	var server string
	if len(*nameserver) == 0 {
		// get root server from local DNS
		conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return "", err
		}
		server = fmt.Sprintf("%s:%s", conf.Servers[0], conf.Port)
	} else {
		server = fmt.Sprintf("%s:53", *nameserver)
	}
	return server, nil
}
