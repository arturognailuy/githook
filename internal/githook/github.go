package githook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Run struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Event      string `json:"event"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}
type Artifact struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Expired   bool      `json:"expired"`
	Digest    string    `json:"digest"`
	ExpiresAt time.Time `json:"expires_at"`
}
type GitHub struct {
	Client                     *http.Client
	BaseURL, Repository, Token string
}

func (g GitHub) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{Timeout: 2 * time.Minute}
}
func (g GitHub) base() string {
	if g.BaseURL != "" {
		return strings.TrimRight(g.BaseURL, "/")
	}
	return "https://api.github.com"
}
func (g GitHub) get(ctx context.Context, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, g.base()+path, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	resp, err := g.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GitHub %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
func (g GitHub) Run(ctx context.Context, id int64) (Run, error) {
	var r Run
	err := g.get(ctx, fmt.Sprintf("/repos/%s/actions/runs/%d", g.Repository, id), &r)
	return r, err
}
func (g GitHub) LatestSuccessfulRun(ctx context.Context, workflowPath, branch string) (Run, error) {
	var v struct {
		Runs []Run `json:"workflow_runs"`
	}
	path := fmt.Sprintf("/repos/%s/actions/workflows/%s/runs?branch=%s&event=push&status=success&per_page=1", g.Repository, url.PathEscape(workflowPath), url.QueryEscape(branch))
	if err := g.get(ctx, path, &v); err != nil {
		return Run{}, err
	}
	if len(v.Runs) != 1 {
		return Run{}, fmt.Errorf("no eligible run")
	}
	return v.Runs[0], nil
}
func (g GitHub) Artifact(ctx context.Context, runID int64, expected string) (Artifact, error) {
	var v struct {
		Artifacts []Artifact `json:"artifacts"`
	}
	if err := g.get(ctx, fmt.Sprintf("/repos/%s/actions/runs/%d/artifacts?per_page=100", g.Repository, runID), &v); err != nil {
		return Artifact{}, err
	}
	var matches []Artifact
	for _, a := range v.Artifacts {
		if a.Name == expected && !a.Expired && a.ExpiresAt.After(time.Now()) {
			matches = append(matches, a)
		}
	}
	if len(matches) != 1 {
		return Artifact{}, permanent(fmt.Errorf("expected one unexpired artifact %q, got %d", expected, len(matches)))
	}
	return matches[0], nil
}
func (g GitHub) Download(ctx context.Context, id int64) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, g.base()+fmt.Sprintf("/repos/%s/actions/artifacts/%d/zip", g.Repository, id), nil)
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artifact download: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<20))
}
func zipBytes(b []byte) *bytes.Reader { return bytes.NewReader(b) }
