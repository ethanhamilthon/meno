package process

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	outputHeadLimit = 32 * 1024
	outputTailLimit = 24 * 1024
	logLimit        = 10 * 1024 * 1024
)

type outputCollector struct {
	prefix []byte
	tail   []byte
	ringAt int
	small  []byte
	total  int64
}

func newOutputCollector() *outputCollector {
	return &outputCollector{small: make([]byte, 0, outputHeadLimit+outputTailLimit)}
}

func (c *outputCollector) add(p []byte) {
	c.total += int64(len(p))
	if c.small != nil {
		if len(c.small)+len(p) <= outputHeadLimit+outputTailLimit {
			c.small = append(c.small, p...)
		} else {
			c.small = nil
		}
	}
	if len(c.prefix) < outputHeadLimit {
		n := min(outputHeadLimit-len(c.prefix), len(p))
		c.prefix = append(c.prefix, p[:n]...)
	}
	c.addTail(p)
}

func (c *outputCollector) addTail(p []byte) {
	if len(p) >= outputTailLimit {
		c.tail = append(c.tail[:0], p[len(p)-outputTailLimit:]...)
		c.ringAt = 0
		return
	}
	for _, b := range p {
		if len(c.tail) < outputTailLimit {
			c.tail = append(c.tail, b)
			continue
		}
		c.tail[c.ringAt] = b
		c.ringAt = (c.ringAt + 1) % outputTailLimit
	}
}

func (c *outputCollector) result(logPath string) (string, int64) {
	if c.small != nil {
		return string(c.small), 0
	}
	tail := c.tail
	if len(tail) == outputTailLimit && c.ringAt != 0 {
		tail = append(append([]byte(nil), tail[c.ringAt:]...), tail[:c.ringAt]...)
	}
	omitted := c.total - int64(len(c.prefix)+len(tail))
	marker := fmt.Sprintf("\n[%d bytes omitted; full output: %s]\n", omitted, logPath)
	return string(c.prefix) + marker + string(tail), omitted
}

type cappedLog struct {
	file *os.File
	size int64
}

func (l *cappedLog) write(p []byte) error {
	if l.file == nil {
		return fmt.Errorf("output log is closed")
	}
	if len(p) == 0 {
		return nil
	}
	if int64(len(p)) >= logLimit {
		p = p[len(p)-logLimit:]
		if err := l.file.Truncate(0); err != nil {
			return err
		}
		if _, err := l.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		_, err := l.file.Write(p)
		l.size = int64(len(p))
		return err
	}
	if l.size+int64(len(p)) > logLimit {
		keep := logLimit - int64(len(p))
		tail := make([]byte, keep)
		if _, err := l.file.ReadAt(tail, l.size-keep); err != nil {
			return err
		}
		if err := l.file.Truncate(0); err != nil {
			return err
		}
		if _, err := l.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := l.file.Write(tail); err != nil {
			return err
		}
		if _, err := l.file.Write(p); err != nil {
			return err
		}
		l.size = logLimit
		return nil
	}
	if _, err := l.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	if _, err := l.file.Write(p); err != nil {
		return err
	}
	l.size += int64(len(p))
	return nil
}

func createLog(dir, callID string) (*cappedLog, string, error) {
	if dir == "" || callID == "" || filepath.Base(callID) != callID || callID == "." || callID == ".." {
		return nil, "", fmt.Errorf("output directory and simple call ID are required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, "", err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, callID+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return nil, "", err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return nil, "", err
	}
	return &cappedLog{file: f}, path, nil
}
