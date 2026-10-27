package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type updateStage struct {
	events *[]string
	fail   bool
}

func (s updateStage) Close() error { *s.events = append(*s.events, "cleanup"); return nil }
func (s updateStage) Commit(context.Context) error {
	*s.events = append(*s.events, "commit")
	if s.fail {
		return errors.New("rename failed")
	}
	return nil
}

func TestUpdateLifecycleOrdering(t *testing.T) {
	for _, tc := range []struct{ name, fail, want string }{
		{"no update", "latest-old", "latest"},
		{"release failure", "latest", "latest"},
		{"prepare failure", "prepare", "latest prepare"},
		{"commit failure", "commit", "latest prepare commit cleanup"},
		{"success", "", "latest prepare commit cleanup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{}
			d := updateDependencies{
				latest: func(context.Context) (string, error) {
					events = append(events, "latest")
					if tc.fail == "latest" {
						return "", errors.New("network")
					}
					if tc.fail == "latest-old" {
						return "v1.0.0", nil
					}
					return "v2.0.0", nil
				},
				prepare: func(context.Context, string, string) (stagedUpdate, error) {
					events = append(events, "prepare")
					if tc.fail == "prepare" {
						return nil, errors.New("checksum")
					}
					return updateStage{&events, tc.fail == "commit"}, nil
				},
			}
			var out bytes.Buffer
			err := performUpdate(context.Background(), &out, "1.0.0", "meno", d)
			if got := strings.Join(events, " "); got != tc.want {
				t.Fatalf("events %s; want %s", got, tc.want)
			}
			wantErr := tc.fail != "" && tc.fail != "latest-old"
			if (err != nil) != wantErr {
				t.Fatalf("error %v", err)
			}
			if err != nil && strings.Contains(out.String(), "updated meno") {
				t.Fatal("false success")
			}
		})
	}
}
