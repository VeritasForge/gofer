// Package slack 은 Incoming Webhook 으로 메시지를 보낸다. 도구 공용.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Post 는 webhookURL 로 text 한 건을 보낸다. 2xx 가 아니면 상태와 본문을 담은 오류를 돌려준다.
// 오류 문구에는 webhookURL 을 담지 않는다 — 이 값은 daily log(0644)에 쓰이는데, config 는 0600 이기 때문이다.
func Post(ctx context.Context, webhookURL, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack webhook: invalid URL: %w", unwrapURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("slack webhook: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("slack webhook: %s: %s", resp.Status, bytes.TrimSpace(b))
	}
	return nil
}

// unwrapURLError 는 *url.Error 의 URL 을 담지 않은 내부 오류를 꺼낸다 (net/http 는 Do/NewRequest 실패를 늘 *url.Error 로 감싼다).
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}
