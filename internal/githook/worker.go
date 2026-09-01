package githook

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"
)

type Worker struct {
	Queue                              *Queue
	GitHub                             GitHub
	Deployer                           Deployer
	WorkflowName, WorkflowPath, Branch string
}

func (w Worker) ProcessRun(ctx context.Context, runID int64, expectedSHA string) error {
	if w.Queue != nil {
		older, err := w.Queue.RefuseOlder(ctx, runID)
		if err != nil {
			return err
		}
		if older {
			return fmt.Errorf("run is older than or equal to deployed state")
		}
	}
	r, err := w.GitHub.Run(ctx, runID)
	if err != nil {
		return err
	}
	if r.ID != runID || r.Repository.FullName != w.GitHub.Repository || r.Name != w.WorkflowName || r.Path != w.WorkflowPath || r.Event != "push" || r.HeadBranch != w.Branch || r.Status != "completed" || r.Conclusion != "success" || !strings.EqualFold(r.HeadSHA, expectedSHA) {
		return fmt.Errorf("run metadata is not eligible")
	}
	name := "site-release-" + strings.ToLower(r.HeadSHA)
	a, err := w.GitHub.Artifact(ctx, runID, name)
	if err != nil {
		return err
	}
	data, err := w.GitHub.Download(ctx, a.ID)
	if err != nil {
		return err
	}
	if strings.HasPrefix(a.Digest, "sha256:") {
		if err := verifyDigest(data, strings.TrimPrefix(a.Digest, "sha256:")); err != nil {
			return err
		}
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	bundle, err := VerifyBundle(z, w.GitHub.Repository, runID, r.HeadSHA)
	if err != nil {
		return err
	}
	if err := w.Deployer.Deploy(ctx, r.HeadSHA, bundle.Archive); err != nil {
		return err
	}
	if w.Queue != nil {
		return w.Queue.MarkDeployed(ctx, runID, r.HeadSHA)
	}
	return nil
}
func verifyDigest(data []byte, want string) error {
	if !strings.EqualFold(sha256Hex(data), want) {
		return fmt.Errorf("GitHub artifact digest mismatch")
	}
	return nil
}
func (w Worker) Run(ctx context.Context) error {
	if err := w.Queue.Recover(ctx); err != nil {
		return err
	}
	for {
		j, err := w.Queue.Claim(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				continue
			}
		}
		if err = w.ProcessRun(ctx, j.RunID, j.HeadSHA); err != nil {
			_ = w.Queue.Retry(ctx, j.DeliveryID, err.Error(), time.Minute)
			continue
		}
		_ = w.Queue.Complete(ctx, j.DeliveryID)
	}
}
