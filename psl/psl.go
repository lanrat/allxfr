// Package psl provides functionality to fetch domains from the Public Suffix List.
package psl

import (
	"context"
	"net/http"
	"time"

	"github.com/miekg/dns"
	"github.com/weppos/publicsuffix-go/publicsuffix"
)

const (
	pslURL     = "https://publicsuffix.org/list/public_suffix_list.dat"
	pslTimeout = 30 * time.Second
)

// GetDomains fetches and parses the Public Suffix List to extract domain names.
// It downloads the PSL from publicsuffix.org, parses the rules (excluding private domains),
// converts IDN domains to ASCII, and returns them as fully qualified domain names.
// The function includes timeout handling to prevent indefinite blocking.
func GetDomains() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pslTimeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", pslURL, nil)
	if err != nil {
		return nil, err
	}
	
	client := &http.Client{Timeout: pslTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	list := publicsuffix.NewList()
	options := &publicsuffix.ParserOption{
		PrivateDomains: false,
	}
	rules, err := list.Load(resp.Body, options)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.Type != publicsuffix.ExceptionType {
			domain, err := publicsuffix.ToASCII(rule.Value)
			if err != nil {
				return out, err
			}
			out = append(out, dns.Fqdn(domain))
		}
	}
	return out, nil
}
