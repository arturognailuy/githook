package githook

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Deployer struct {
	ReleasesDir, CurrentLink string
	SmokeURLs                []string
	Client                   *http.Client
}

func (d Deployer) Deploy(ctx context.Context, sha string, archive []byte) error {
	if !validSHA(sha) {
		return fmt.Errorf("invalid release SHA")
	}
	target := filepath.Join(d.ReleasesDir, strings.ToLower(sha))
	if _, e := os.Lstat(target); !os.IsNotExist(e) {
		return fmt.Errorf("release already exists")
	}
	tmp, err := os.MkdirTemp(d.ReleasesDir, ".incoming-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err = extractTarGz(archive, tmp); err != nil {
		return err
	}
	if err = sealRelease(tmp); err != nil {
		return err
	}
	if err = os.Rename(tmp, target); err != nil {
		return err
	}
	previous, _ := os.Readlink(d.CurrentLink)
	newLink := d.CurrentLink + ".new"
	_ = os.Remove(newLink)
	if err = os.Symlink(target, newLink); err != nil {
		return err
	}
	if err = os.Rename(newLink, d.CurrentLink); err != nil {
		return err
	}
	if err = d.smoke(ctx); err != nil {
		if previous != "" {
			rollback := d.CurrentLink + ".rollback"
			_ = os.Remove(rollback)
			if e := os.Symlink(previous, rollback); e == nil {
				_ = os.Rename(rollback, d.CurrentLink)
			}
		}
		return fmt.Errorf("smoke check failed and activation rolled back: %w", err)
	}
	return nil
}
func extractTarGz(data []byte, dir string) error {
	gz, e := gzip.NewReader(bytes.NewReader(data))
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		if !safeTarPath(h.Name, h.Typeflag) {
			return fmt.Errorf("unsafe path %q", h.Name)
		}
		dst := filepath.Join(dir, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if e = os.MkdirAll(dst, 0755); e != nil {
				return e
			}
		case tar.TypeReg:
			if e = os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
				return e
			}
			f, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, h.FileInfo().Mode().Perm()&0755)
			if e != nil {
				return e
			}
			_, e = io.Copy(f, io.LimitReader(tr, 128<<20))
			ce := f.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
		default:
			return fmt.Errorf("unsafe entry type")
		}
	}
}
func sealRelease(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(path, 0555)
		}
		return os.Chmod(path, 0444)
	})
}

func (d Deployer) smoke(ctx context.Context) error {
	c := d.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	for _, u := range d.SmokeURLs {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		r, e := c.Do(req)
		if e != nil {
			return e
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
		r.Body.Close()
		if r.StatusCode < 200 || r.StatusCode >= 400 {
			return fmt.Errorf("%s returned %s", u, r.Status)
		}
	}
	return nil
}
