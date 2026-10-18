package notify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"
)

func TestPresetsAreDistinctValidWAV(t *testing.T) {
	names := Names()
	if len(names) != 25 {
		t.Fatalf("got %d sounds", len(names))
	}
	seen := map[[32]byte]bool{}
	for _, name := range names {
		data, err := WAV(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(data[:4]) != "RIFF" || string(data[8:16]) != "WAVEfmt " || string(data[36:40]) != "data" {
			t.Fatalf("%s: invalid header", name)
		}
		if int(binary.LittleEndian.Uint32(data[40:44])) != len(data)-44 {
			t.Fatal("incorrect data length")
		}
		if int(binary.LittleEndian.Uint32(data[4:8])) != len(data)-8 {
			t.Fatal("incorrect RIFF length")
		}
		if len(data) < 4000 || len(data) > 88244 {
			t.Fatalf("%s: unexpected duration", name)
		}
		if bytes.Count(data[44:], []byte{0}) == len(data)-44 {
			t.Fatal("silent sound")
		}
		hash := sha256.Sum256(data)
		if seen[hash] {
			t.Fatalf("duplicate sound: %s", name)
		}
		seen[hash] = true
	}
	names[0] = "changed"
	if Names()[0] == "changed" {
		t.Fatal("mutable names")
	}
	if _, err := WAV("../../bad"); err == nil {
		t.Fatal("unknown name accepted")
	}
	if err := Play(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown playback accepted")
	}
}

func TestPlayerSelection(t *testing.T) {
	for _, tc := range []struct {
		os, available string
		wantArgs      int
	}{
		{"darwin", "afplay", 0}, {"linux", "paplay", 0}, {"linux", "aplay", 0}, {"linux", "ffplay", 4},
	} {
		path, args, err := findPlayer(tc.os, func(name string) (string, error) {
			if name == tc.available {
				return "/bin/" + name, nil
			}
			return "", errors.New("missing")
		})
		if err != nil || path != "/bin/"+tc.available || len(args) != tc.wantArgs {
			t.Fatalf("%+v: %q %v %v", tc, path, args, err)
		}
	}
	if _, _, err := findPlayer("linux", func(string) (string, error) { return "", errors.New("missing") }); err == nil {
		t.Fatal("missing player accepted")
	}
}
