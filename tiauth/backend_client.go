package tiauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PrivateHost is the loopback address used for the private server.
// Uses 127.0.0.2 for isolation from the main loopback (127.0.0.1).
const PrivateHost = "127.0.0.2"

// BackendClient provides HTTP communication with the Python backend's private server.
type BackendClient struct {
	baseURL string
	client  *http.Client
}

// NewBackendClient creates a new HTTP client for the Python backend.
func NewBackendClient(port int) *BackendClient {
	return &BackendClient{
		baseURL: fmt.Sprintf("http://%s:%d", PrivateHost, port),
		client:  &http.Client{},
	}
}

// SendActionInvocationEndpointRequest implements faroe's ActionInvocationEndpointClientInterface
// by sending JSON requests to Python's /invoke endpoint.
func (c *BackendClient) SendActionInvocationEndpointRequest(requestJSON string) (string, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/invoke", bytes.NewReader([]byte(requestJSON)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

// SendTestNotification sends a token notification to Python's /token endpoint.
// This is used for testing when SMTP is disabled.
func (c *BackendClient) SendTestNotification(action, email, code string) error {
	payload := map[string]string{
		"action": action,
		"email":  email,
		"code":   code,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/token", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token notification: %w", err)
	}
	defer resp.Body.Close()

	// Always read the response body to ensure the HTTP transaction completes
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token notification failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
