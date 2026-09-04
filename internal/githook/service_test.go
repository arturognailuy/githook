package githook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceRoutesWebhookAndHidesUnknownPaths(t *testing.T) {
	q, err := OpenQueue(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	r := Receiver{Secret: []byte("s"), Repository: "gnailuy/gnailuy.com", Queue: q}
	s := Service{Receiver: r, Queue: q}
	body := `{"repository":{"full_name":"gnailuy/gnailuy.com"}}`
	req := httptest.NewRequest(http.MethodPost, DefaultWebhookPath, http.NoBody)
	req.Body = ioNopCloser(body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Delivery", "12345678-1234-1234-1234-123456789abc")
	req.Header.Set("X-Hub-Signature-256", sign(body, "s"))
	response := httptest.NewRecorder()
	s.ServeHTTP(response, req)
	if response.Code != http.StatusOK || response.Body.String() != "42\n" {
		t.Fatalf("webhook status=%d body=%q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusNotFound || response.Body.String() != "42\n" {
		t.Fatalf("unknown status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestServiceQueueMaintenance(t *testing.T) {
	q, err := OpenQueue(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	ctx := context.Background()
	sha := "0123456789012345678901234567890123456789"
	ids := []string{"12345678-1234-1234-1234-123456789abc", "22345678-1234-1234-1234-123456789abc", "32345678-1234-1234-1234-123456789abc"}
	for i, id := range ids {
		if added, err := q.Enqueue(ctx, id, int64(i+1), sha); err != nil || !added {
			t.Fatalf("enqueue added=%v err=%v", added, err)
		}
	}
	s := Service{Queue: q}

	response := httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, queuePath+"/peek", nil))
	var peek Job
	if err := json.Unmarshal(response.Body.Bytes(), &peek); err != nil || peek.RunID != 3 {
		t.Fatalf("peek=%+v err=%v", peek, err)
	}

	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, queuePath+"/"+ids[0], nil))
	if response.Code != http.StatusOK {
		t.Fatalf("drop status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest(http.MethodPost, queuePath+"/keep-one", nil))
	jobs, err := q.Pending(ctx)
	if err != nil || len(jobs) != 1 || jobs[0].RunID != 3 {
		t.Fatalf("pending=%+v err=%v", jobs, err)
	}

	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, queuePath, nil))
	jobs, err = q.Pending(ctx)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("pending after clear=%+v err=%v", jobs, err)
	}
}

type stringReadCloser struct{ *strings.Reader }

func (stringReadCloser) Close() error           { return nil }
func ioNopCloser(value string) stringReadCloser { return stringReadCloser{strings.NewReader(value)} }
