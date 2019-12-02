package main

import (
	"compress/gzip"
	"github.com/miekg/dns"
	"io"
	"log"
	"os"
	"strings"
)

func parseZoneFile(filename string) (zone, error) {
	var z zone
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
	log.Printf("parsing zonefile: %s\n", filename)
	zp := dns.NewZoneParser(fileReader, "", "")
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		z.AddRecord(rr)
	}

	if err := zp.Err(); err != nil {
		return z, err
	}
	log.Printf("zonefile parsing done")
	return z, nil
}
