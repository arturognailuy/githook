package githook

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type Manifest struct {
	Repository    string `json:"repository"`
	WorkflowRunID int64  `json:"workflow_run_id"`
	HeadSHA       string `json:"head_sha"`
	Archive       string `json:"archive"`
}
type Bundle struct {
	Manifest Manifest
	Archive  []byte
}

func VerifyBundle(z *zip.Reader, repository string, runID int64, sha string) (Bundle, error) {
	files := map[string][]byte{}
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !safePath(f.Name) || f.Mode()&os.ModeType != 0 {
			return Bundle{}, fmt.Errorf("unsafe bundle path %q", f.Name)
		}
		if _, exists := files[f.Name]; exists {
			return Bundle{}, fmt.Errorf("duplicate bundle path %q", f.Name)
		}
		rc, e := f.Open()
		if e != nil {
			return Bundle{}, e
		}
		b, e := io.ReadAll(io.LimitReader(rc, 512<<20))
		rc.Close()
		if e != nil {
			return Bundle{}, e
		}
		files[f.Name] = b
	}
	var m Manifest
	if err := json.Unmarshal(files["manifest.json"], &m); err != nil {
		return Bundle{}, fmt.Errorf("manifest: %w", err)
	}
	if m.Repository != repository || m.WorkflowRunID != runID || !strings.EqualFold(m.HeadSHA, sha) || m.Archive == "" || !safePath(m.Archive) {
		return Bundle{}, fmt.Errorf("manifest does not match verified run")
	}
	archive, ok := files[m.Archive]
	if !ok {
		return Bundle{}, fmt.Errorf("archive %q missing", m.Archive)
	}
	fields := strings.Fields(string(files[m.Archive+".sha256"]))
	if len(fields) < 1 {
		return Bundle{}, fmt.Errorf("checksum missing")
	}
	want, e := hex.DecodeString(fields[0])
	if e != nil || len(want) != sha256.Size {
		return Bundle{}, fmt.Errorf("invalid checksum")
	}
	sum := sha256.Sum256(archive)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), hex.EncodeToString(want)) {
		return Bundle{}, fmt.Errorf("checksum mismatch")
	}
	if err := InspectTarGz(archive); err != nil {
		return Bundle{}, err
	}
	return Bundle{Manifest: m, Archive: archive}, nil
}
func safePath(name string) bool {
	return name != "" && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}
func InspectTarGz(data []byte) error {
	gz, e := gzip.NewReader(bytes.NewReader(data))
	if e != nil {
		return fmt.Errorf("archive gzip: %w", e)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return fmt.Errorf("unsafe archive entry %q type %d", h.Name, h.Typeflag)
		}
		name := h.Name
		if h.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if !safePath(name) {
			return fmt.Errorf("unsafe archive path %q", h.Name)
		}
	}
	return nil
}
