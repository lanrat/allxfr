package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lanrat/allxfr/resolver"
	"github.com/lanrat/allxfr/zone"

	"github.com/lanrat/allxfr/psl"

	"golang.org/x/sync/errgroup"
)

var (
	parallel  = flag.Uint("parallel", 10, "number of parallel zone transfers to perform")
	saveDir   = flag.String("out", "zones", "directory to save found zones in")
	verbose   = flag.Bool("verbose", false, "enable verbose output")
	zonefile  = flag.String("zonefile", "", "use the provided zonefile instead of getting the root zonefile")
	saveAll   = flag.Bool("save-all", false, "attempt AXFR from every nameserver for a given zone and save all answers")
	usePSL    = flag.Bool("psl", false, "attempt AXFR from zones listed in the public suffix list, requires -ns flag")
	ixfr      = flag.Bool("ixfr", false, "attempt an IXFR instead of AXFR")
	dryRun    = flag.Bool("dry-run", false, "only test if xfr is allowed by retrieving one envelope")
	retry     = flag.Int("retry", 3, "number of times to retry failed operations")
	overwrite = flag.Bool("overwrite", false, "if zone already exists on disk, overwrite it with newer data")
)

var (
	totalXFR uint32
	query    resolver.Resolver
)

const (
	globalTimeout = 15 * time.Second
)

func main() {
	flag.Parse()
	if *retry < 1 {
		log.Fatal("retry must be positive")
	}
	if flag.NArg() > 0 {
		log.Fatalf("unexpected arguments %v", flag.Args())
	}
	query = *resolver.NewWithTimeout(globalTimeout)

	start := time.Now()
	var z zone.Zone
	var err error
	if len(*zonefile) == 0 {
		// get zone file from root AXFR
		// not all the root nameservers allow AXFR, try them until we find one that does
		for _, ns := range resolver.RootServerNames {
			v("trying root nameserver %s", ns)
			startTime := time.Now()
			z, err = zone.RootAXFR(ns)
			if err == nil {
				took := time.Since(startTime).Round(time.Millisecond)
				log.Printf("ROOT %s xfr size: %d records in %s \n", ns, z.Records, took.String())
				break
			}
		}
	} else {
		// zone file is provided
		v("parsing zonefile: %q\n", *zonefile)
		z, err = zone.ParseZoneFile(*zonefile)
		check(err)
	}

	if z.CountNS() == 0 {
		log.Fatal("Got empty zone")
	}

	if *usePSL {
		pslDomains, err := psl.GetDomains()
		check(err)
		for _, domain := range pslDomains {
			z.AddNS(domain, "")
		}
		v("added %d domains from PSL\n", len(pslDomains))
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
