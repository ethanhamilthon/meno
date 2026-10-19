package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const maxBinary = 200 << 20

// Asset says which files of a release to use: the list of checksums, the
// archive for this platform, and the file to take out of the archive. meno and
// the cliproxyapi helper are both installed this way.
type Asset struct {
	Sums   string // name of the checksum list, for example "SHA256SUMS"
	Suffix string // end of the archive name for this platform
	Binary string // base name of the file inside the archive
}

// Install downloads the release tag for this platform, checks its SHA-256
// against SHA256SUMS and replaces the executable at exe with it.
func (c Client) Install(ctx context.Context, tag, exe string) error {
	return c.install(ctx, tag, exe, runtime.GOOS, runtime.GOARCH)
}

func (c Client) install(ctx context.Context, tag, exe, goos, goarch string) error {
	return c.InstallAsset(ctx, tag, Asset{Sums: "SHA256SUMS", Suffix: "_" + goos + "_" + goarch + ".tar.gz", Binary: "meno"}, exe)
}

// InstallAsset downloads the archive of the release tag that matches a, checks
// it against the checksum list and writes the binary from it to dest, replacing
// a file that is there. A failed download leaves dest as it was.
func (c Client) InstallAsset(ctx context.Context, tag string, a Asset, dest string) error {
	base := c.downloadBase(tag)
	sums, err := c.fetchAll(ctx, base+"/"+a.Sums)
	if err != nil {
		return fmt.Errorf("cannot download the checksums of %s: %w", tag, err)
	}
	name, want := findArchive(string(sums), a.Suffix)
	if name == "" {
		return fmt.Errorf("release %s has no build for %s", tag, strings.TrimSuffix(strings.TrimPrefix(a.Suffix, "_"), ".tar.gz"))
	}
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".meno-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (%w); reinstall with install.sh or rerun with the needed rights", dir, err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	resp, err := c.get(ctx, base+"/"+name)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	sum := sha256.New()
	if err := extract(io.TeeReader(resp.Body, sum), tmp, a.Binary); err != nil {
		return err
	}
	// The tar reader stops at the end of the archive: take in the rest so the
	// hash covers every byte.
	_, _ = io.Copy(io.Discard, io.TeeReader(resp.Body, sum))
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// findArchive returns the archive in a SHA256SUMS listing whose name ends in
// suffix, with its hash.
func findArchive(sums, suffix string) (name, hash string) {
	sc := bufio.NewScanner(strings.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.HasSuffix(f[1], suffix) {
			return f[1], strings.ToLower(f[0])
		}
	}
	return "", ""
}

func (c Client) fetchAll(ctx context.Context, url string) ([]byte, error) {
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// extract copies the file called binary of a .tar.gz stream into w.
func extract(r io.Reader, w io.Writer, binary string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("the archive has no %s binary", binary)
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == binary {
			n, err := io.Copy(w, io.LimitReader(tr, maxBinary+1))
			if err == nil && n > maxBinary {
				err = errors.New("the binary is unreasonably large")
			}
			return err
		}
	}
}
