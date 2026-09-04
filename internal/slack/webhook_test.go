package slack

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostSendsTextAsJSON(t *testing.T) {
	var gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(b), r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()
	if err := Post(t.Context(), srv.URL, "line 1\nline \"2\""); err != nil {
		t.Fatal(err)
	}
	if gotType != "application/json" || gotBody != `{"text":"line 1\nline \"2\""}` {
		t.Errorf("type=%q body=%q", gotType, gotBody)
	}
}

func TestPostReportsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "invalid_payload")
	}))
	defer srv.Close()
	err := Post(t.Context(), srv.URL, "x")
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "invalid_payload") {
		t.Fatalf("got %v", err)
	}
}
