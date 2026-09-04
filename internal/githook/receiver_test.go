package githook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeQueue struct {
	added bool
	calls int
	err   error
}

func (f *fakeQueue) Enqueue(_ context.Context, _ string, _ int64, _ string) (bool, error) {
	f.calls++
	return f.added, f.err
}
func sign(body, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}
func request(t *testing.T, r Receiver, method, event, body, sig string) *httptest.ResponseRecorder {
	t.Helper()
	q := httptest.NewRequest(method, "/", strings.NewReader(body))
	q.Header.Set("Content-Type", "application/json")
	q.Header.Set("X-GitHub-Event", event)
	q.Header.Set("X-GitHub-Delivery", "12345678-1234-1234-1234-123456789abc")
	q.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, q)
	return w
}
func TestReceiverWorkflowRunAndDedup(t *testing.T) {
	body := `{"action":"completed","repository":{"full_name":"gnailuy/gnailuy.com"},"workflow_run":{"id":42,"head_sha":"0123456789012345678901234567890123456789"}}`
	f := &fakeQueue{added: true}
	r := Receiver{Secret: []byte("test-secret"), Repository: "gnailuy/gnailuy.com", Queue: f}
	if got := request(t, r, http.MethodPost, "workflow_run", body, sign(body, "test-secret")).Code; got != http.StatusAccepted {
		t.Fatalf("got %d", got)
	}
	f.added = false
	if got := request(t, r, http.MethodPost, "workflow_run", body, sign(body, "test-secret")).Code; got != http.StatusOK {
		t.Fatalf("dedup got %d", got)
	}
	if f.calls != 2 {
		t.Fatal("queue not called")
	}
}
func TestReceiverSecurityChecks(t *testing.T) {
	body := `{"repository":{"full_name":"gnailuy/gnailuy.com"}}`
	r := Receiver{Secret: []byte("test-secret"), Repository: "gnailuy/gnailuy.com", Queue: &fakeQueue{}}
	tests := []struct {
		name, method, event, sig string
		want                     int
	}{{"method", http.MethodGet, "ping", sign(body, "test-secret"), 405}, {"signature", http.MethodPost, "ping", "sha256=00", 401}, {"event", http.MethodPost, "push", sign(body, "test-secret"), 422}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := request(t, r, tt.method, tt.event, body, tt.sig)
			if response.Code != tt.want || response.Body.String() != "42\n" {
				t.Fatalf("got status=%d body=%q want status=%d", response.Code, response.Body.String(), tt.want)
			}
		})
	}
}
func TestReceiverPingAndLimits(t *testing.T) {
	body := `{"repository":{"full_name":"gnailuy/gnailuy.com"}}`
	r := Receiver{Secret: []byte("s"), Repository: "gnailuy/gnailuy.com", Queue: &fakeQueue{}, MaxBody: int64(len(body))}
	response := request(t, r, http.MethodPost, "ping", body, sign(body, "s"))
	if response.Code != http.StatusOK || response.Body.String() != "42\n" {
		t.Fatalf("got status=%d body=%q", response.Code, response.Body.String())
	}
	r.MaxBody = 1
	if got := request(t, r, http.MethodPost, "ping", body, sign(body, "s")).Code; got != 413 {
		t.Fatalf("got %d", got)
	}
}
func TestListenAndServeRejectsNonLoopback(t *testing.T) {
	err := ListenAndServe(context.Background(), "0.0.0.0:20182", http.NotFoundHandler())
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("got %v", err)
	}
}

func TestReceiverQueueFailure(t *testing.T) {
	body := `{"action":"completed","repository":{"full_name":"gnailuy/gnailuy.com"},"workflow_run":{"id":42,"head_sha":"0123456789012345678901234567890123456789"}}`
	r := Receiver{Secret: []byte("s"), Repository: "gnailuy/gnailuy.com", Queue: &fakeQueue{err: errors.New("down")}}
	if got := request(t, r, http.MethodPost, "workflow_run", body, sign(body, "s")).Code; got != 503 {
		t.Fatalf("got %d", got)
	}
}
