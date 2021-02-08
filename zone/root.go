package zone

import (
	"fmt"

	"github.com/miekg/dns"
)

// GetRootServers returns the DNS root servers
func GetRootServers(nameserver string) ([]string, error) {
	out := make([]string, 0, 4)
	// get root servers
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeNS)
	in, err := dns.Exchange(m, nameserver)
	if err != nil {
		return out, err
	}
	for _, a := range in.Answer {
		if ns, ok := a.(*dns.NS); ok {
			out = append(out, ns.Ns)
		}
	}
	if len(out) == 0 {
		return out, fmt.Errorf("unable to find root server")

	}
	return out, nil
}

// RootAXFR returns a Zone containing the ROOT zone
func RootAXFR(ns string) (Zone, error) {
	m := new(dns.Msg)
	m.SetQuestion(".", dns.TypeAXFR)
	t := new(dns.Transfer)

	var root Zone
	env, err := t.In(m, fmt.Sprintf("%s:53", ns))
	if err != nil {
		return root, fmt.Errorf("transfer error from %v: %w", ns, err)
	}
	for e := range env {
		if e.Error != nil {
			return root, fmt.Errorf("transfer envelope error from %v: %w", ns, e.Error)
		}
		for _, r := range e.RR {
			root.AddRecord(r)
		}
	}
	return root, nil
}
