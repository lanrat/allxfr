package save

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/miekg/dns"
)

// File represents the zone file to create on disk
type File struct {
	filename    string
	filenameTmp string
	zone        string
	bufWriter   *bufio.Writer
	gzWriter    *gzip.Writer
	fileWriter  *os.File
	records     int64
	closed      bool
}

// New returns a handle to a new zonefile
func New(zone, filename string) *File {
	f := new(File)
	f.filename = filename
	f.filenameTmp = fmt.Sprintf("%s.tmp", f.filename)
	f.zone = zone
	return f
}

// Records returns the number of records written to the zone file
func (f *File) Records() int64 {
	return f.records
}

// WriteComment adds a comment to the zone file
func (f *File) WriteComment(comment string) error {
	err := f.fileReady()
	if err != nil {
		return err
	}
	_, err = f.bufWriter.WriteString(fmt.Sprintf("; %s", comment))
	return err
}

// WriteCommentKey adds a comment to the zone file
func (f *File) WriteCommentKey(key, value string) error {
	return f.WriteComment(fmt.Sprintf("%s: %s\n", key, value))
}

// ErrFileClosed returned when attempting to write to a closed file
var ErrFileClosed = errors.New("file is already closed")

// fileReady internal function to ensure that the file is ready before data can be written
// safe to call multiple times
func (f *File) fileReady() error {
	var err error
	if f.closed {
		return ErrFileClosed
	}
	if f.bufWriter == nil {
		f.fileWriter, err = os.Create(f.filenameTmp)
		if err != nil {
			return err
		}
		f.gzWriter = gzip.NewWriter(f.fileWriter)
		f.gzWriter.ModTime = time.Now()
		f.gzWriter.Name = fmt.Sprintf("%s.zone", f.zone[:len(f.zone)-1])
		f.bufWriter = bufio.NewWriter(f.gzWriter)
		// Save metadata to zone file as comment
		err = f.WriteCommentKey("timestamp", time.Now().Format(time.RFC3339))
		if err != nil {
			return err
		}
		err = f.WriteCommentKey("zone", f.zone)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddRR adds a record to a zone file
func (f *File) AddRR(rr dns.RR) error {
	// create file here on first rr
	err := f.fileReady()
	if err != nil {
		return err
	}

	_, err = f.bufWriter.WriteString(fmt.Sprintf("%s\n", RRString(rr)))
	if err != nil {
		return err
	}
	f.records++
	return nil
}

// Abort stops processing the new zone file and removes it from disk
func (f *File) Abort() error {
	f.records = 0 // forces finish to remove the file
	return f.Finish()
}

// Finish adds closing comments and flushes and closes all buffers/files
func (f *File) Finish() error {
	if f.closed {
		return nil
	}
	// function to finish/close/safe the files when done
	if f.records > 1 {
		// save record count comment at end of zone file
		err := f.WriteCommentKey("records", fmt.Sprintf("%d", f.records))
		if err != nil {
			return err
		}
	}
	var err error
	if f.bufWriter != nil {
		err = f.bufWriter.Flush()
		if err != nil {
			return err
		}
		err = f.gzWriter.Flush()
		if err != nil {
			return err
		}
		err = f.gzWriter.Close()
		if err != nil {
			return err
		}
		err = f.fileWriter.Close()
		if err != nil {
			return err
		}
	}
	if f.records > 1 {
		err = os.Rename(f.filenameTmp, f.filename)
	} else {
		err = os.Remove(f.filenameTmp)
	}
	if err != nil {
		return err
	}
	f.closed = true
	return nil
}
