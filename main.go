package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lanrat/allxfr/resolver"
	"github.com/lanrat/allxfr/status"
	"github.com/lanrat/allxfr/zone"

	"github.com/lanrat/allxfr/psl"

	"golang.org/x/sync/errgroup"
)

var (
	parallel    = flag.Uint("parallel", 10, "number of parallel zone transfers to perform")
	saveDir     = flag.String("out", "zones", "directory to save found zones in")
	verbose     = flag.Bool("verbose", false, "enable verbose output")
	zonefile    = flag.String("zonefile", "", "use the provided zonefile instead of getting the root zonefile")
	saveAll     = flag.Bool("save-all", false, "attempt AXFR from every nameserver for a given zone and save all answers")
	usePSL      = flag.Bool("psl", false, "attempt AXFR from zones listed in the public suffix list")
	ixfr        = flag.Bool("ixfr", false, "attempt an IXFR instead of AXFR")
	dryRun      = flag.Bool("dry-run", false, "only test if xfr is allowed by retrieving one envelope")
	retry       = flag.Int("retry", 3, "number of times to retry failed operations")
	overwrite   = flag.Bool("overwrite", false, "if zone already exists on disk, overwrite it with newer data")
	statusPort  = flag.String("status-port", "", "enable HTTP status server on specified port (e.g., '8080')")
	showVersion = flag.Bool("version", false, "print version and exit") // Show version

)

var (
	version      = "dev" // Version string, set at build time
	totalXFR     uint32
	resolve      *resolver.Resolver
	statusServer *status.StatusServer
)

const (
	globalTimeout = 15 * time.Second
)

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *retry < 1 {
		log.Fatal("retry must be positive")
	}

	// Start HTTP status server if port is specified
	if *statusPort != "" {
		statusServer = status.StartStatusServer(*statusPort)
	}

	resolve = resolver.New()
	start := time.Now()
	var z zone.Zone
	var err error

	if len(*zonefile) > 1 {
		// zone file is provided
		v("parsing zonefile: %q\n", *zonefile)
		z, err = zone.ParseZoneFile(*zonefile)
		check(err)
	} else if len(*zonefile) == 0 && flag.NArg() == 0 {
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
	}

	if flag.NArg() > 0 {
		for _, domain := range flag.Args() {
			z.AddNS(domain, "")
		}
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

	if statusServer != nil {
		statusServer.IncrementTotalZones(uint32(z.CountNS()))
	}

	// create output dir if does not exist
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

		// Update status server with new domain discovered
		if statusServer != nil {
			statusServer.StartTransfer(domain)
		}

		err := axfrWorker(z, domain)
		if err != nil {
			if statusServer != nil {
				statusServer.FailTransfer(domain, err.Error())
			}
			continue
		}

		// If no error occurred, the domain processing is complete
		// Success/failure status is handled within axfr function based on actual transfers
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
