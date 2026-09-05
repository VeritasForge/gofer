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

// TestPostDoesNotLeakURLOnTransportError 는 전송 계층 실패(연결 거부 등) 시 오류 문구에 webhook URL 이
// 담기지 않는지 본다 — net/http 는 이 실패를 *url.Error 로 감싸는데, 그 Error() 는 URL 전체를 그대로 담는다.
func TestPostDoesNotLeakURLOnTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // 닫힌 서버 = 연결 거부(connection refused)

	url := srv.URL + "/services/T/B/SECRET"
	err := Post(t.Context(), url, "x")
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error leaks webhook path: %v", err)
	}
	if strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error leaks webhook host: %v", err)
	}
}
