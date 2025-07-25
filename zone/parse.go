// Package zone provides functionality for parsing and managing DNS zone files.
package zone

import (
	"compress/gzip"
	"io"
	"os"
	"strings"

	"github.com/miekg/dns"
)

// ParseZoneFile parses a DNS zone file and returns a Zone containing the records.
// It supports both plain text and gzip-compressed zone files (detected by .gz extension).
// The function extracts NS, A, and AAAA records to build the zone structure.
func ParseZoneFile(filename string) (Zone, error) {
	var z Zone
	var fileReader io.Reader
	file, err := os.Open(filename)
	fileReader = file
	if err != nil {
		return z, err
	}
	defer func() { _ = file.Close() }()
	if strings.HasSuffix(filename, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return z, err
		}
		fileReader = gz
		defer func() { _ = gz.Close() }()
	}
	zp := dns.NewZoneParser(fileReader, "", "")
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		z.AddRecord(rr)
	}

	if err := zp.Err(); err != nil {
		return z, err
	}
	return z, nil
}
