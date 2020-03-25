package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

var (
	parallel = flag.Uint("parallel", 10, "number of parallel zone transfers to perform")
	saveDir  = flag.String("out", "zones", "directory to save found zones in")
	verbose  = flag.Bool("verbose", false, "enable verbose output")
	zonefile = flag.String("zonefile", "", "use the provided zonefile instead of getting the root zonefile")
	ns       = flag.String("ns", "", "nameserver to use for manualy querying of records not in zone file")
	saveAll  = flag.Bool("save-all", false, "attempt AXFR from every nameserver for a given zone and save all answers")
	psl      = flag.Bool("psl", false, "attempt AXFR from zones listed in the public suffix list, requires -ns flag")
	ixfr     = flag.Bool("ixfr", false, "attempt an IXFR instead of AXFR")
	dryRun   = flag.Bool("dry-run", false, "only test if xfr is allowed by retrieving one envelope")
)

var (
	localNameserver string
	totalXFR        uint32
)

const (
	globalTimeout = 5 * time.Second
)

func main() {
	log.SetFlags(0)
	flag.Parse()
	var err error
	localNameserver, err = getNameserver()
	check(err)
	if *verbose {
		log.Printf("Using initial nameserver %s", localNameserver)
	}

	start := time.Now()
	var z zone
	if len(*zonefile) == 0 {
		rootNameservers, err := getRootServers()
		check(err)
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

	if *psl {
		pslDomains, err := getPSLDomsins()
		check(err)
		for _, domain := range pslDomains {
			z.AddNS(domain, "")
		}
		if *verbose {
			log.Printf("added %d domains from PSL\n", len(pslDomains))
		}
	}

	// create outpout dir if does not exist
	if !*dryRun {
		if _, err := os.Stat(*saveDir); os.IsNotExist(err) {
			err = os.MkdirAll(*saveDir, os.ModePerm)
			check(err)
		}
	}

	if *verbose {
		z.PrintTree()
	}

	zoneChan := z.GetNameChan()
	var g errgroup.Group

	// start workers
	for i := uint(0); i < *parallel; i++ {
		g.Go(func() error { return worker(z, zoneChan) })
	}

	err = g.Wait()
	check(err)
	took := time.Since(start).Round(time.Millisecond)
	log.Printf("%d / %d transferred in %s\n", totalXFR, len(z.ns), took.String())
	if *verbose {
		log.Printf("exiting normally\n")
	}
}

func worker(z zone, c chan string) error {
	for {
		domain, more := <-c
		if !more {
			return nil
		}
		err := axfrWorker(z, domain)
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

// getNameserver returns the nameserver passed via flag if provided, if not returns the system's NS
func getNameserver() (string, error) {
	var server string
	if len(*ns) == 0 {
		// get root server from local DNS
		conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return "", err
		}
		server = net.JoinHostPort(conf.Servers[0], conf.Port)
	} else {
		host, port, err := net.SplitHostPort(*ns)
		if err != nil {
			server = net.JoinHostPort(*ns, "53")
		} else {
			server = net.JoinHostPort(host, port)
		}
	}
	return server, nil
}
