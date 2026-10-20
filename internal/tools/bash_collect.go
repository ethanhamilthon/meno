package tools

import (
	"errors"
	"io"
	"os"
)

func (c *Call) collect(reader *os.File, log *cappedLog, path string) {
	defer reader.Close()
	defer log.file.Close()
	defer close(c.collected)
	collector := newOutputCollector()
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			collector.add(chunk)
			if writeErr := log.write(chunk); writeErr != nil && c.err == nil {
				c.err = writeErr
			}
			if c.options.Stream != nil {
				c.options.Stream(string(chunk))
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && c.err == nil {
				c.err = err
			}
			break
		}
	}
	c.result.Output, c.result.Omitted = collector.result(path)
	c.result.LogPath = path
}
