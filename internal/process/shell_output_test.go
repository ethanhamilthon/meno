package process

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestOutputCollectorCutShape(t *testing.T) {
	collector := newOutputCollector()
	input := bytes.Repeat([]byte("x"), outputHeadLimit+outputTailLimit+123)
	collector.add(input)
	text, omitted := collector.result("/output/call.log")
	marker := "\n[123 bytes omitted; full output: /output/call.log]\n"
	if !strings.HasPrefix(text, strings.Repeat("x", outputHeadLimit)+marker) {
		t.Fatal("missing head or marker")
	}
	if !strings.HasSuffix(text, strings.Repeat("x", outputTailLimit)) {
		t.Fatal("missing tail")
	}
	if omitted != 123 {
		t.Fatalf("omitted = %d", omitted)
	}
}

func TestCappedLogKeepsNewestBytes(t *testing.T) {
	dir := t.TempDir()
	log, path, err := createLog(dir, "call")
	if err != nil {
		t.Fatal(err)
	}
	defer log.file.Close()
	first := bytes.Repeat([]byte("a"), logLimit-10)
	second := bytes.Repeat([]byte("b"), 20)
	if err := log.write(first); err != nil {
		t.Fatal(err)
	}
	if err := log.write(second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != logLimit {
		t.Fatalf("size = %d", len(data))
	}
	if !bytes.Equal(data[len(data)-20:], second) || !bytes.Equal(data[:10], bytes.Repeat([]byte("a"), 10)) {
		t.Fatal("log did not retain newest bytes")
	}
}
