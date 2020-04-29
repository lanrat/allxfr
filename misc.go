package main

import (
	"bufio"
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

// writeComment adds a zone file comment to the writer w
func writeComment(w *bufio.Writer, key, value string) error {
	_, err := w.WriteString(fmt.Sprintf("; %s: %s\n", key, value))
	return err
}
