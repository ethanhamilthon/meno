package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Prepared is a verified executable staged on its destination filesystem.
// Close removes uncommitted staging; Commit atomically replaces the destination.
type Prepared struct{ path, dest string }

func (p *Prepared) Close() error {
	err := os.Remove(p.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (p *Prepared) Commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(p.path, p.dest)
}

// Prepare stages meno without replacing the installed executable.
func (c Client) Prepare(ctx context.Context, tag, dest string) (*Prepared, error) {
	if resolved, err := filepath.EvalSymlinks(dest); err == nil {
		dest = resolved
	}
	return c.PrepareAsset(ctx, tag, Asset{Sums: "SHA256SUMS", Suffix: "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz", Binary: "meno"}, dest)
}

// PrepareAsset downloads, verifies and stages an asset without installing it.
func (c Client) PrepareAsset(ctx context.Context, tag string, a Asset, dest string) (*Prepared, error) {
	base := c.downloadBase(tag)
	sums, err := c.fetchAll(ctx, base+"/"+a.Sums)
	if err != nil {
		return nil, fmt.Errorf("cannot download the checksums of %s: %w", tag, err)
	}
	name, want := findArchive(string(sums), a.Suffix)
	if name == "" {
		return nil, fmt.Errorf("release %s has no build for %s", tag, strings.TrimSuffix(strings.TrimPrefix(a.Suffix, "_"), ".tar.gz"))
	}
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(dir, ".meno-update-*")
	if err != nil {
		return nil, fmt.Errorf("cannot write to %s (%w); reinstall with install.sh or rerun with the needed rights", dir, err)
	}
	ready := false
	defer func() {
		if !ready {
			_ = os.Remove(tmp.Name())
		}
	}()
	defer tmp.Close()
	resp, err := c.get(ctx, base+"/"+name)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	sum := sha256.New()
	if err := extract(io.TeeReader(resp.Body, sum), tmp, a.Binary); err != nil {
		return nil, err
	}
	// The tar reader stops at the end of the archive: take in the rest so the
	// hash covers every byte.
	if _, err := io.Copy(io.Discard, io.TeeReader(resp.Body, sum)); err != nil {
		return nil, err
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return nil, fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ready = true
	return &Prepared{path: tmp.Name(), dest: dest}, nil
}
