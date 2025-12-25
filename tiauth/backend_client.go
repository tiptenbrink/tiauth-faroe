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

// EmailRequest represents an email request to the Python backend.
type EmailRequest struct {
	Type        string `json:"type"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
	Code        string `json:"code,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	NewEmail    string `json:"newEmail,omitempty"`
}

// SendEmail sends an email request to Python's /email endpoint.
// The Python backend handles token storage and SMTP delivery.
func (c *BackendClient) SendEmail(req EmailRequest) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/email", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create email request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send email request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("email request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
