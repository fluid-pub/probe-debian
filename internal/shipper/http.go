package shipper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	orgUUID    string
	token      string
	httpClient *http.Client
}

func New(baseURL, orgUUID, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		orgUUID: orgUUID,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Register(ctx context.Context) error {
	url := fmt.Sprintf("%s/probes/register/%s/%s", c.baseURL, c.orgUUID, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("register failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) PushState(ctx context.Context, payload map[string]interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/probes/v1/ingest/%s/%s", c.baseURL, c.orgUUID, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("push state failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/probes/ping/%s/%s", c.baseURL, c.orgUUID, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ping failed with status %d", resp.StatusCode)
	}
	return nil
}
