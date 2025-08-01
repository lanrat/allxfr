// Package save provides functionality for writing DNS zone files to disk with compression.
// It handles the creation, writing, and finalization of zone files, including
// automatic gzip compression, metadata comments, and atomic file operations
// to ensure data integrity during the zone transfer process.
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

// File represents a zone file being written to disk with compression.
// It manages the lifecycle of creating, writing, and finalizing DNS zone files.
type File struct {
	filename      string        // Final filename for the zone file
	filenameTmp   string        // Temporary filename during writing
	zone          string        // Zone name being written
	bufWriter     *bufio.Writer // Buffered writer for performance
	gzWriter      *gzip.Writer  // Gzip compression writer
	fileWriter    *os.File      // Underlying file writer
	records       int64         // Number of DNS records written
	closed        bool          // Whether the file has been finalized
	pendingWrites []string      // Comments buffered until first record is added
}

// New creates a new zone file writer for the specified zone and filename.
// The file is not created until the first record is written, allowing
// comments to be buffered efficiently.
func New(zone, filename string) *File {
	f := new(File)
	f.filename = filename
	f.filenameTmp = fmt.Sprintf("%s.tmp", f.filename)
	f.zone = zone
	return f
}

// Records returns the number of DNS records written to the zone file.
func (f *File) Records() int64 {
	return f.records
}

// WriteComment adds a comment to the zone file
func (f *File) WriteComment(comment string) error {
	if f.closed {
		return ErrFileClosed
	}
	// If file isn't created yet, buffer the comment
	if f.bufWriter == nil {
		f.pendingWrites = append(f.pendingWrites, fmt.Sprintf("; %s", comment))
		return nil
	}
	// File is already created, write directly
	_, err := fmt.Fprintf(f.bufWriter, "; %s", comment)
	return err
}

// WriteCommentKey writes a key-value comment pair to the zone file.
// The comment is formatted as "; key: value\n".
func (f *File) WriteCommentKey(key, value string) error {
	return f.WriteComment(fmt.Sprintf("%s: %s\n", key, value))
}

// ErrFileClosed is returned when attempting to write to a file that has been closed.
var ErrFileClosed = errors.New("file is already closed")

// fileReady ensures the file is created and ready for writing.
// It creates the temporary file, sets up compression, and writes initial metadata.
// This function is safe to call multiple times.
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

		// Write timestamp and zone metadata
		_, err = fmt.Fprintf(f.bufWriter, "; timestamp: %s\n", time.Now().Format(time.RFC3339))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(f.bufWriter, "; zone: %s\n", f.zone)
		if err != nil {
			return err
		}

		// Write all buffered comments
		for _, comment := range f.pendingWrites {
			_, err = fmt.Fprintf(f.bufWriter, "%s", comment)
			if err != nil {
				return err
			}
		}
		// Clear the buffer
		f.pendingWrites = nil
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

	_, err = fmt.Fprintf(f.bufWriter, "%s\n", rr.String())
	if err != nil {
		return err
	}
	f.records++
	return nil
}

// Abort cancels the zone file creation and removes any temporary files.
// It sets the record count to 0 to ensure the file is deleted rather than saved.
func (f *File) Abort() error {
	f.records = 0 // forces finish to remove the file
	return f.Finish()
}

// Finish adds closing comments and flushes and closes all buffers/files
func (f *File) Finish() error {
	if f.closed {
		return nil
	}

	// If no file was created (no records were added), just mark as closed
	if f.bufWriter == nil {
		f.closed = true
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
	if f.records > 0 {
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
