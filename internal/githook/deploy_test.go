package githook

import (
	"archive/tar"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarGzAcceptsDirectoryWithTrailingSlash(t *testing.T) {
	root := t.TempDir()
	if err := extractTarGz(tarball(t, "about/", tar.TypeDir), root); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "about")); err != nil || !info.IsDir() {
		t.Fatalf("directory not extracted: info=%v err=%v", info, err)
	}
}

func TestDeployAndRollback(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0755)
			}
			return nil
		})
	})
	releases := filepath.Join(root, "releases")
	os.Mkdir(releases, 0755)
	old := filepath.Join(releases, "old")
	os.Mkdir(old, 0755)
	current := filepath.Join(root, "current")
	os.Symlink(old, current)
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer good.Close()
	sha := "0123456789012345678901234567890123456789"
	d := Deployer{ReleasesDir: releases, CurrentLink: current, SmokeURLs: []string{good.URL}}
	if err := d.Deploy(context.Background(), sha, tarball(t, "index.html", tar.TypeReg)); err != nil {
		t.Fatal(err)
	}
	target, _ := os.Readlink(current)
	if target == old {
		t.Fatal("not activated")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer bad.Close()
	sha2 := "1123456789012345678901234567890123456789"
	d.SmokeURLs = []string{bad.URL}
	if err := d.Deploy(context.Background(), sha2, tarball(t, "index.html", tar.TypeReg)); err == nil {
		t.Fatal("expected smoke failure")
	}
	rolled, _ := os.Readlink(current)
	if rolled != target {
		t.Fatalf("not rolled back: %s", rolled)
	}
}
