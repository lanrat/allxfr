// Package save provides utilities for saving DNS zone files.
package save

import (
	"fmt"

	"github.com/miekg/dns"
)

// RRString prints IPv4 IPs in AAAA records in IPv6 notation
// fixes https://github.com/miekg/dns/issues/1107
func RRString(rr dns.RR) string {
	if aaaa, ok := rr.(*dns.AAAA); ok {
		ipStr := aaaa.AAAA.String()
		if aaaa.AAAA.To4() != nil {
			ipStr = fmt.Sprintf("::ffff:%s", ipStr)
		}
		return aaaa.Hdr.String() + ipStr
	}
	return rr.String()
}
