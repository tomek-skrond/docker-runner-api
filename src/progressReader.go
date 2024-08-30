package main

import (
	"io"
	"log"
)

// ProgressReader wraps an io.Reader to log the progress of reading data
type ProgressReader struct {
	Reader       io.Reader
	TotalBytes   int64
	LoggedBytes  int64
	Logger       *log.Logger
	NextLogPoint int64
}

// Read overrides the Read method to add progress logging
func (p *ProgressReader) Read(b []byte) (int, error) {
	n, err := p.Reader.Read(b)
	if n > 0 {
		p.LoggedBytes += int64(n)
		percentage := float64(p.LoggedBytes) / float64(p.TotalBytes) * 100

		if p.NextLogPoint == 0 {
			p.NextLogPoint = 5
		}

		if percentage >= float64(p.NextLogPoint) {
			p.Logger.Printf("Uploaded %.0f%%", percentage)
			p.NextLogPoint += 5
		}
	}

	return n, err
}
