package cutover

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestClientDecodesBusinessErrorBeforeStatus(t *testing.T) {
	client := NewClient("test-key")
	client.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected request: %s", request.Method)
		}
		body := `{"ok":false,"data":null,"error":{"code":"INVALID_ARGUMENT","message":"domain is required"},"metadata":{}}`
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	_, err := client.ZoneID(context.Background(), "example.com")
	apiErr, ok := err.(*InfraiError)
	if !ok || apiErr.Code != "INVALID_ARGUMENT" || apiErr.HTTPStatus != 400 {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestClientRetriesRateLimit(t *testing.T) {
	attempts := 0
	client := NewClient("test-key")
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	client.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		attempts++
		status, body := 429, `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`
		if attempts == 2 {
			status, body = 200, `{"ok":true,"data":{"zone_id":"zone_123"},"error":null,"metadata":{}}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Retry-After": {"0"}}}, nil
	})
	zoneID, err := client.ZoneID(context.Background(), "example.com")
	if err != nil || zoneID != "zone_123" || attempts != 2 {
		t.Fatalf("zoneID=%q attempts=%d err=%v", zoneID, attempts, err)
	}
}
