package grafana

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

type Client struct {
	url       string
	publicURL string
	adminUser string
	adminPass string
	client    *http.Client
}

func NewClient(cfg config.GrafanaConfig) *Client {
	return &Client{
		url:       cfg.URL,
		publicURL: cfg.PublicURL,
		adminUser: cfg.AdminUser,
		adminPass: cfg.AdminPassword,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type GrafanaUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
}

func (c *Client) CreateUser(email, name, password string) (*GrafanaUser, error) {
	body := map[string]interface{}{
		"name":     name,
		"email":    email,
		"login":    email,
		"password": password,
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", c.url+"/api/admin/users", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.adminUser, c.adminPass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("grafana create user request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusPreconditionFailed {
		// User already exists
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("grafana create user failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	return &GrafanaUser{ID: result.ID, Login: email}, nil
}

func (c *Client) CreateDatasource(name, bucket, influxURL, influxToken, org string) error {
	body := map[string]interface{}{
		"name":   name,
		"type":   "influxdb",
		"access": "proxy",
		"url":    influxURL,
		"jsonData": map[string]interface{}{
			"version":        "Flux",
			"organization":   org,
			"defaultBucket":  bucket,
		},
		"secureJsonData": map[string]interface{}{
			"token": influxToken,
		},
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", c.url+"/api/datasources", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.adminUser, c.adminPass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("grafana create datasource request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil // Already exists
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("grafana create datasource failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) GetDashboardURL(bucket string) string {
	return fmt.Sprintf("%s/d/k6-metrics?var-bucket=%s", c.publicURL, bucket)
}

func (c *Client) PublicURL() string {
	return c.publicURL
}

func (c *Client) Ping() error {
	req, err := http.NewRequest("GET", c.url+"/api/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("grafana unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana returned status %d", resp.StatusCode)
	}
	return nil
}

// PingWithToken tests connectivity using a service account token (Bearer auth).
func (c *Client) PingWithToken(token string) error {
	req, err := http.NewRequest("GET", c.url+"/api/org", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("grafana unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("grafana token invalid (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana returned status %d", resp.StatusCode)
	}
	return nil
}
