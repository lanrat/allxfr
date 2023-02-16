package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/lanrat/allxfr/zone"

	"github.com/lanrat/allxfr/psl"

	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
)

var (
	parallel  = flag.Uint("parallel", 10, "number of parallel zone transfers to perform")
	saveDir   = flag.String("out", "zones", "directory to save found zones in")
	verbose   = flag.Bool("verbose", false, "enable verbose output")
	zonefile  = flag.String("zonefile", "", "use the provided zonefile instead of getting the root zonefile")
	ns        = flag.String("ns", "", "nameserver to use for manually querying of records not in zone file")
	saveAll   = flag.Bool("save-all", false, "attempt AXFR from every nameserver for a given zone and save all answers")
	usePSL    = flag.Bool("psl", false, "attempt AXFR from zones listed in the public suffix list, requires -ns flag")
	ixfr      = flag.Bool("ixfr", false, "attempt an IXFR instead of AXFR")
	dryRun    = flag.Bool("dry-run", false, "only test if xfr is allowed by retrieving one envelope")
	retry     = flag.Int("retry", 3, "number of times to retry failed operations")
	overwrite = flag.Bool("overwrite", false, "if zone already exists on disk, overwrite it with newer data")
	root      = flag.Bool("root", false, "axfr all the root zones")
)

var (
	localNameserver string
	totalXFR        uint32
)

const (
	globalTimeout = 15 * time.Second
)

func main() {
	//log.SetFlags(0)
	flag.Parse()
	if *usePSL && len(*ns) == 0 {
		log.Fatal("must pass nameserver with -ns when using -psl")
	}
	if *retry < 1 {
		log.Fatal("retry must be positive")
	}
	// if flag.NArg() > 0 {
	// 	log.Fatalf("unexpected arguments %v", flag.Args())
	// }
	var err error
	localNameserver, err = getNameserver()
	check(err)
	v("using initial nameserver %s", localNameserver)

	start := time.Now()
	var z zone.Zone
	if *root {
		rootNameservers, err := zone.GetRootServers(localNameserver)
		check(err)
		// get zone file from root AXFR
		// not all the root nameservers allow AXFR, try them until we find one that does
		for _, ns := range rootNameservers {
			v("trying root nameserver %s", ns)
			startTime := time.Now()
			z, err = zone.RootAXFR(ns)
			if err == nil {
				took := time.Since(startTime).Round(time.Millisecond)
				log.Printf("ROOT %s xfr size: %d records in %s \n", ns, z.Records, took.String())
				break
			}
		}
	}
	if len(*zonefile) > 0 {
		// zone file is provided
		v("parsing zonefile: %q\n", *zonefile)
		z, err = zone.ParseZoneFile(*zonefile)
		check(err)
	}

	for _, domain := range flag.Args() {
		z.AddNS(domain, "")
	}

	if *usePSL {
		pslDomains, err := psl.GetDomains()
		check(err)
		for _, domain := range pslDomains {
			z.AddNS(domain, "")
		}
		v("added %d domains from PSL\n", len(pslDomains))
	}

	if z.CountNS() == 0 {
		log.Fatal("Nothing to do")
	}

	// create outpout dir if does not exist
	// TODO fix dry run causing panics on save
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
	log.Printf("%d / %d transferred in %s\n", totalXFR, len(z.NS), took.String())
	v("exiting normally\n")
}

func worker(z zone.Zone, c chan string) error {
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

func v(format string, v ...interface{}) {
	if *verbose {
		line := fmt.Sprintf(format, v...)
		lines := strings.ReplaceAll(line, "\n", "\n\t")
		log.Print(lines)
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
