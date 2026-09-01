package githook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
)

const DefaultMaxBody = int64(1 << 20)

var deliveryPattern = regexp.MustCompile(`^[0-9a-fA-F-]{16,64}$`)

type Enqueuer interface {
	Enqueue(context.Context, string, int64, string) (bool, error)
}
type Receiver struct {
	Secret     []byte
	Repository string
	Queue      Enqueuer
	MaxBody    int64
}
type eventEnvelope struct {
	Action     string `json:"action"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	WorkflowRun struct {
		ID      int64  `json:"id"`
		HeadSHA string `json:"head_sha"`
	} `json:"workflow_run"`
}

func (r Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	max := r.MaxBody
	if max <= 0 {
		max = DefaultMaxBody
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, max+1))
	if err != nil || int64(len(body)) > max {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	if !validSignature(body, req.Header.Get("X-Hub-Signature-256"), r.Secret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	delivery := req.Header.Get("X-GitHub-Delivery")
	if !deliveryPattern.MatchString(delivery) {
		http.Error(w, "invalid delivery id", http.StatusBadRequest)
		return
	}
	var event eventEnvelope
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if event.Repository.FullName != r.Repository {
		http.Error(w, "unexpected repository", http.StatusForbidden)
		return
	}
	switch req.Header.Get("X-GitHub-Event") {
	case "ping":
		w.WriteHeader(http.StatusNoContent)
	case "workflow_run":
		if event.Action != "completed" || event.WorkflowRun.ID <= 0 || !validSHA(event.WorkflowRun.HeadSHA) {
			http.Error(w, "unsupported workflow event", http.StatusUnprocessableEntity)
			return
		}
		added, err := r.Queue.Enqueue(req.Context(), delivery, event.WorkflowRun.ID, strings.ToLower(event.WorkflowRun.HeadSHA))
		if err != nil {
			http.Error(w, "queue unavailable", http.StatusServiceUnavailable)
			return
		}
		if added {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	default:
		http.Error(w, "unsupported event", http.StatusUnprocessableEntity)
	}
}
func validSignature(body []byte, header string, secret []byte) bool {
	if len(secret) == 0 || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	want, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil || len(want) != sha256.Size {
		return false
	}
	m := hmac.New(sha256.New, secret)
	_, _ = m.Write(body)
	return hmac.Equal(m.Sum(nil), want)
}
func validSHA(s string) bool { _, err := hex.DecodeString(s); return len(s) == 40 && err == nil }
func ListenAndServe(ctx context.Context, addr string, h http.Handler) error {
	s := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5e9, ReadTimeout: 10e9, WriteTimeout: 10e9, IdleTimeout: 30e9, MaxHeaderBytes: 16 << 10}
	go func() { <-ctx.Done(); _ = s.Shutdown(context.Background()) }()
	err := s.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve: %w", err)
}
