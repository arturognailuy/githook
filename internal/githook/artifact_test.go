package githook

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func tarball(t *testing.T, name string, kind byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	data := []byte("ok")
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: kind}); err != nil {
		t.Fatal(err)
	}
	if kind == tar.TypeReg {
		tw.Write(data)
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}
func bundleZip(t *testing.T, archive []byte, m Manifest, checksum string) *zip.Reader {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	mb, _ := json.Marshal(m)
	for n, data := range map[string][]byte{"manifest.json": mb, m.Archive: archive, m.Archive + ".sha256": []byte(checksum + "  " + m.Archive + "\n")} {
		f, _ := zw.Create(n)
		f.Write(data)
	}
	zw.Close()
	z, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return z
}
func TestVerifyBundle(t *testing.T) {
	sha := "0123456789012345678901234567890123456789"
	a := tarball(t, "index.html", tar.TypeReg)
	sum := sha256.Sum256(a)
	m := Manifest{Repository: "example/project", WorkflowRunID: 7, HeadSHA: sha, Archive: "bundle.tar.gz"}
	b, err := VerifyBundle(bundleZip(t, a, m, hex.EncodeToString(sum[:])), m.Repository, 7, sha)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Archive) == 0 {
		t.Fatal("archive missing")
	}
}
func TestVerifyBundleRejectsTamperAndTraversal(t *testing.T) {
	sha := "0123456789012345678901234567890123456789"
	m := Manifest{Repository: "example/project", WorkflowRunID: 7, HeadSHA: sha, Archive: "bundle.tar.gz"}
	a := tarball(t, "index.html", tar.TypeReg)
	if _, err := VerifyBundle(bundleZip(t, a, m, "00"), m.Repository, 7, sha); err == nil {
		t.Fatal("accepted bad checksum")
	}
	bad := tarball(t, "../escape", tar.TypeReg)
	sum := sha256.Sum256(bad)
	if _, err := VerifyBundle(bundleZip(t, bad, m, hex.EncodeToString(sum[:])), m.Repository, 7, sha); err == nil {
		t.Fatal("accepted traversal")
	}
	link := tarball(t, "link", tar.TypeSymlink)
	sum = sha256.Sum256(link)
	if _, err := VerifyBundle(bundleZip(t, link, m, hex.EncodeToString(sum[:])), m.Repository, 7, sha); err == nil {
		t.Fatal("accepted link")
	}
}
