package psl

import (
	"net/http"

	"github.com/miekg/dns"
	"github.com/weppos/publicsuffix-go/publicsuffix"
)

const pslURL = "https://publicsuffix.org/list/public_suffix_list.dat"

func GetDomains() ([]string, error) {
	resp, err := http.Get(pslURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
