package zone

import (
	"fmt"

	"github.com/miekg/dns"
)

// RootAXFR performs a zone transfer against a root nameserver to obtain the root zone.
// It connects to the specified nameserver on port 53 and requests the root zone (".").
// Returns a Zone containing all the NS, A, and AAAA records from the root zone.
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
