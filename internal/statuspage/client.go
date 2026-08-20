package statuspage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://www.githubstatus.com/api/v2"
	userAgent      = "kagutsuchi/0.1.0"
	httpTimeout    = 15 * time.Second
)

// Client queries the GitHub Statuspage API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a new Client. Pass an empty string to use the default
// GitHub statuspage base URL.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// GetStatus returns the overall status indicator from /api/v2/status.json.
func (c *Client) GetStatus() (*StatusInfo, error) {
	var res struct {
		Status StatusInfo `json:"status"`
	}
	if err := c.get("/status.json", &res); err != nil {
		return nil, err
	}
	return &res.Status, nil
}

// GetComponents returns all components from /api/v2/components.json.
func (c *Client) GetComponents() ([]Component, error) {
	var res struct {
		Components []Component `json:"components"`
	}
	if err := c.get("/components.json", &res); err != nil {
		return nil, err
	}
	return res.Components, nil
}

// GetIncidents returns incidents from the given page of /api/v2/incidents.json.
func (c *Client) GetIncidents(page int) ([]Incident, error) {
	path := fmt.Sprintf("/incidents.json?page=%d", page)
	var res struct {
		Incidents []Incident `json:"incidents"`
	}
	if err := c.get(path, &res); err != nil {
		return nil, err
	}
	return res.Incidents, nil
}

// get performs a GET request and unmarshals the JSON response into dest.
func (c *Client) get(path string, dest interface{}) error {
	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("statuspage: create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("statuspage: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("statuspage: GET %s: %s (status %d)", path, string(body[:min(len(body), 256)]), resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("statuspage: decode %s: %w", path, err)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
