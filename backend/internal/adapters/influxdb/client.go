package influxdb

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
	url    string
	token  string
	org    string
	orgID  string
	client *http.Client
}

func NewClient(cfg config.InfluxDBConfig) *Client {
	return &Client{
		url:   cfg.URL,
		token: cfg.Token,
		org:   cfg.Org,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type bucket struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	OrgID string `json:"orgID"`
}

type bucketsResponse struct {
	Buckets []bucket `json:"buckets"`
}

func (c *Client) CreateBucket(name string) error {
	orgID, err := c.getOrgID()
	if err != nil {
		return fmt.Errorf("failed to get org ID: %w", err)
	}

	body := map[string]interface{}{
		"name":  name,
		"orgID": orgID,
		"retentionRules": []map[string]interface{}{
			{"type": "expire", "everySeconds": 0},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.url+"/api/v2/buckets", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnprocessableEntity {
		// Bucket may already exist
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create bucket (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) DeleteBucket(name string) error {
	b, err := c.GetBucketByName(name)
	if err != nil {
		return err
	}
	if b == nil {
		return nil
	}

	req, err := http.NewRequest("DELETE", c.url+"/api/v2/buckets/"+b.ID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete bucket (status %d)", resp.StatusCode)
	}

	return nil
}

func (c *Client) GetBucketByName(name string) (*bucket, error) {
	req, err := http.NewRequest("GET", c.url+"/api/v2/buckets?name="+name, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get bucket (status %d)", resp.StatusCode)
	}

	var result bucketsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for _, b := range result.Buckets {
		if b.Name == name {
			return &b, nil
		}
	}

	return nil, nil
}

func (c *Client) ListBuckets(prefix string) ([]string, error) {
	req, err := http.NewRequest("GET", c.url+"/api/v2/buckets?limit=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result bucketsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var names []string
	for _, b := range result.Buckets {
		if prefix == "" || len(b.Name) >= len(prefix) && b.Name[:len(prefix)] == prefix {
			names = append(names, b.Name)
		}
	}

	return names, nil
}

func (c *Client) ClearBucket(name string) error {
	orgID, err := c.getOrgID()
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"start": "1970-01-01T00:00:00Z",
		"stop":  "2100-01-01T00:00:00Z",
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v2/delete?org=%s&bucket=%s", c.url, c.org, name),
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to clear bucket (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	_ = orgID
	return nil
}

func (c *Client) getOrgID() (string, error) {
	if c.orgID != "" {
		return c.orgID, nil
	}

	req, err := http.NewRequest("GET", c.url+"/api/v2/orgs?org="+c.org, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Orgs []struct {
			ID string `json:"id"`
		} `json:"orgs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Orgs) == 0 {
		return "", fmt.Errorf("organization '%s' not found", c.org)
	}

	c.orgID = result.Orgs[0].ID
	return c.orgID, nil
}

func (c *Client) URL() string  { return c.url }
func (c *Client) Token() string { return c.token }
func (c *Client) Org() string   { return c.org }
