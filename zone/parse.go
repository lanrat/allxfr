package zone

import (
	"compress/gzip"
	"io"
	"os"
	"strings"

	"github.com/miekg/dns"
)

// ParseZoneFile parses the provided zonefile into a Zone
func ParseZoneFile(filename string) (Zone, error) {
	var z Zone
	var fileReader io.Reader
	file, err := os.Open(filename)
	fileReader = file
	if err != nil {
		return z, err
	}
	defer file.Close()
	if strings.HasSuffix(filename, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return z, err
		}
		fileReader = gz
		defer gz.Close()
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
