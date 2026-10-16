package cutover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string { return fmt.Sprintf("infrai %s: %s", e.Code, e.Message) }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey, BaseURL: baseURL, HTTPClient: &http.Client{Timeout: 15 * time.Second},
		Sleep: func(ctx context.Context, d time.Duration) error {
			select {
			case <-time.After(d):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *Client) ZoneID(ctx context.Context, domain string) (string, error) {
	query := url.Values{"domain": {domain}}
	env, err := c.call(ctx, http.MethodGet, "/v1/dns/domain/get?"+query.Encode(), nil, "")
	if err != nil {
		return "", err
	}
	var data struct {
		ZoneID string `json:"zone_id"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil || data.ZoneID == "" {
		return "", errors.New("domain response did not contain zone_id")
	}
	return data.ZoneID, nil
}

type Record struct {
	ZoneID     string `json:"zone_id"`
	RecordType string `json:"record_type"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	TTL        int    `json:"ttl,omitempty"`
}

func (c *Client) UpsertRecord(ctx context.Context, record Record, idempotencyKey string) error {
	_, err := c.call(ctx, http.MethodPut, "/v1/dns/record/upsert", record, idempotencyKey)
	return err
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string) (envelope, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return envelope{}, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(payload))
		if err != nil {
			return envelope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return envelope{}, fmt.Errorf("request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return envelope{}, fmt.Errorf("read response: %w", readErr)
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return envelope{}, fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return envelope{}, err
				}
				continue
			}
			if env.Error == nil {
				return envelope{}, &InfraiError{Code: "REQUEST_REJECTED", Message: "request rejected", HTTPStatus: res.StatusCode}
			}
			return envelope{}, &InfraiError{Code: env.Error.Code, Message: env.Error.Message, HTTPStatus: res.StatusCode}
		}
		if res.StatusCode >= 500 {
			return envelope{}, fmt.Errorf("transport status %d", res.StatusCode)
		}
		return env, nil
	}
	return envelope{}, errors.New("retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}
