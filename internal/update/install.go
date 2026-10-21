package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
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
	p, err := c.PrepareAsset(ctx, tag, a, dest)
	if err != nil {
		return err
	}
	defer p.Close()
	return p.Commit(ctx)
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
