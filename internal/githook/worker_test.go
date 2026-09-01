package githook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func eligibleRun() Run {
	var r Run
	r.ID = 42
	r.Name = "Verify site"
	r.Path = ".github/workflows/verify.yml"
	r.Event = "push"
	r.HeadBranch = "master"
	r.HeadSHA = "0123456789012345678901234567890123456789"
	r.Status = "completed"
	r.Conclusion = "success"
	r.Repository.FullName = "gnailuy/gnailuy.com"
	return r
}

func TestWorkerRejectsIneligibleMetadata(t *testing.T) {
	tests := map[string]func(*Run){
		"repository":    func(r *Run) { r.Repository.FullName = "attacker/repo" },
		"workflow name": func(r *Run) { r.Name = "Other" },
		"workflow path": func(r *Run) { r.Path = ".github/workflows/other.yml" },
		"pull request":  func(r *Run) { r.Event = "pull_request" },
		"branch":        func(r *Run) { r.HeadBranch = "feature" },
		"failed":        func(r *Run) { r.Conclusion = "failure" },
		"unfinished":    func(r *Run) { r.Status = "in_progress" },
		"sha":           func(r *Run) { r.HeadSHA = strings.Repeat("1", 40) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			r := eligibleRun()
			mutate(&r)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(r) }))
			defer s.Close()
			worker := Worker{GitHub: GitHub{BaseURL: s.URL, Repository: "gnailuy/gnailuy.com"}, WorkflowName: "Verify site", WorkflowPath: ".github/workflows/verify.yml", Branch: "master"}
			if err := worker.ProcessRun(context.Background(), 42, eligibleRun().HeadSHA); err == nil {
				t.Fatal("accepted ineligible run")
			}
		})
	}
}

func TestGitHubRequiresExactlyOneUnexpiredArtifact(t *testing.T) {
	future := time.Now().Add(time.Hour)
	artifacts := []Artifact{{ID: 1, Name: "site-release-sha", ExpiresAt: future}, {ID: 2, Name: "other", ExpiresAt: future}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"artifacts": artifacts})
	}))
	defer s.Close()
	g := GitHub{BaseURL: s.URL, Repository: "gnailuy/gnailuy.com"}
	if a, err := g.Artifact(context.Background(), 42, "site-release-sha"); err != nil || a.ID != 1 {
		t.Fatalf("artifact=%+v err=%v", a, err)
	}
	artifacts = append(artifacts, Artifact{ID: 3, Name: "site-release-sha", ExpiresAt: future})
	if _, err := g.Artifact(context.Background(), 42, "site-release-sha"); err == nil {
		t.Fatal("accepted duplicate artifacts")
	}
}
