package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHelpGolden(t *testing.T) {
	for _, topic := range Topics() {
		got, err := Text(topic)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", topic+".golden")
		if os.Getenv("UPDATE_GOLDEN") != "" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != got {
			t.Errorf("%s help differs from golden file", topic)
		}
	}
}
