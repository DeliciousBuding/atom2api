package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

var upstreamClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:    50,
		IdleConnTimeout: 5 * time.Minute,
	},
}

type UpstreamRequest struct {
	Body    []byte
	Token   string
	URL     string
	Stream  bool
}

func ForwardRequest(ctx context.Context, ur *UpstreamRequest) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", ur.URL, bytes.NewReader(ur.Body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ur.Token)
	req.Header.Set("User-Agent", "atomcode/4.22.0")

	resp, err := upstreamClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func ForwardModelsRequest(ctx context.Context, url, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "atomcode/4.22.0")
	return upstreamClient.Do(req)
}

func ReadAndClose(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
