package www

import "io"

// ProxyReader type is used to know received bytes
type ProxyReader struct {
	io.Reader
	total    int64 // Total # of bytes transferred
	listener func(int64)
}

func (reader *ProxyReader) Read(p []byte) (int, error) {
	n, err := reader.Reader.Read(p)
	reader.total += int64(n)

	if reader.listener != nil {
		go reader.listener(int64(n))
	}

	return n, err
}

// SetReadListener method sets a listener for ProxyReader
func (reader *ProxyReader) SetReadListener(listener func(int64)) {
	reader.listener = listener
}

// Total method returns the number of bytes received
func (reader *ProxyReader) Total() int64 {
	return reader.total
}
