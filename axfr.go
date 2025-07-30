package main

import (
	"context"
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

// ErrAxfrUnsupported indicates that the nameserver does not support AXFR requests.
// This is a common response when zone transfers are disabled on the server.
var ErrAxfrUnsupported = errors.New("AXFR Unsupported")

// ErrorAxfrUnsupportedWrap wraps DNS errors with ErrAxfrUnsupported when
// the error indicates that AXFR is refused or not authorized.
// It specifically handles DNS response codes 5 (Refused) and 9 (Not Authorized).
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

// axfrWorker attempts zone transfers for a domain using all available nameservers and IPs.
// It tries both glue records from the zone data and performs additional NS queries
// to discover non-glue nameserver IPs. Returns nil if any transfer succeeds.
func axfrWorker(ctx context.Context, z zone.Zone, domain string) error {
	attemptedIPs := make(map[string]bool)
	domain = dns.Fqdn(domain)
	var err error
	//var records int64
	var anySuccess bool
	for _, nameserver := range z.NS[domain] {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		for _, ip := range z.IP[nameserver] {
			ipString := ip.To16().String()
			if !attemptedIPs[ipString] {
				attemptedIPs[ipString] = true
				anySuccess, err = axfrRetry(ctx, ip, domain, nameserver)
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
		result, err := resolve.Resolve(ctx, domain, dns.TypeNS)
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
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	for _, nameserver := range qNameservers {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var qIPs []net.IP
		for try := 0; try < *retry; try++ {
			qIPs, err = resolve.LookupIPAll(ctx, nameserver)
			if err != nil {
				v("[%s] %s", domain, err)
			} else {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}

		for _, ip := range qIPs {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			ipString := ip.To16().String()
			if !attemptedIPs[ipString] {
				attemptedIPs[ipString] = true
				v("[%s] trying non-glue AXFR: %s %s", domain, nameserver, ip.String())
				anySuccess, err = axfrRetry(ctx, ip, domain, nameserver)
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

// axfrRetry attempts a zone transfer with retry logic.
// It retries failed transfers up to the configured retry count, but skips
// retries if the nameserver explicitly doesn't support AXFR.
// Returns (success, error) where success indicates if any records were transferred.
func axfrRetry(ctx context.Context, ip net.IP, domain, nameserver string) (bool, error) {
	var err error
	var records int64
	var anySuccess bool

	for try := 0; try < *retry; try++ {
		records, err = axfr(ctx, domain, nameserver, ip)
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
		select {
		case <-ctx.Done():
			return anySuccess, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	if !*saveAll && records != 0 {
		return anySuccess, nil
	}
	if err != nil {
		return anySuccess, err
	}

	return anySuccess, err
}

// axfr performs a single zone transfer attempt and logs the result.
// It calls axfrToFile to perform the actual transfer and updates global
// transfer statistics and status server on success.
// Returns the number of records transferred.
func axfr(ctx context.Context, domain, nameserver string, ip net.IP) (int64, error) {
	startTime := time.Now()
	records, err := axfrToFile(ctx, domain, ip, nameserver)
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

// InContext performs a DNS zone transfer using the provided context and connection address.
// It creates a TCP connection with context support and returns a channel of DNS envelopes
// containing the transferred zone records. The transfer respects the context's cancellation
// and uses global timeout settings for the connection operations.
// It wraps miekg/dns.Transfer.In() with a Context
func InContext(ctx context.Context, q *dns.Msg, a string) (env chan *dns.Envelope, err error) {
	// Create a dialer with context
	dialer := &net.Dialer{
		Timeout: globalTimeout,
	}

	// Dial with context
	conn, err := dialer.DialContext(ctx, "tcp", a)
	if err != nil {
		return nil, err
	}

	// Create DNS connection wrapper
	dnsConn := &dns.Conn{Conn: conn}

	// Create transfer with pre-configured connection
	transfer := &dns.Transfer{
		Conn:         dnsConn,
		DialTimeout:  globalTimeout,
		ReadTimeout:  globalTimeout,
		WriteTimeout: globalTimeout,
	}
	return transfer.In(q, a)
}

// axfrToFile performs an AXFR or IXFR request and saves the results to a compressed file.
// It handles file creation, DNS transfer setup with timeouts, and processes each
// envelope of records. Returns the number of records transferred or -1 if the file
// already exists and overwrite is disabled.
func axfrToFile(ctx context.Context, zone string, ip net.IP, nameserver string) (int64, error) {
	zone = dns.Fqdn(zone)

	m := new(dns.Msg)
	if *ixfr {
		m.SetIxfr(zone, 0, "", "")
	} else {
		m.SetQuestion(zone, dns.TypeAXFR)
	}

	env, err := InContext(ctx, m, net.JoinHostPort(ip.String(), "53"))
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
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return zonefile.Records(), ctx.Err()
		default:
		}
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
