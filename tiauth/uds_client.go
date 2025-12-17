package tiauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

// UDSClient provides HTTP communication over Unix domain sockets.
// Connection is established lazily on first request.
type UDSClient struct {
	socketPath string
	client     *http.Client
	mu         sync.Mutex
}

// NewUDSClient creates a new HTTP client for Unix domain sockets.
func NewUDSClient(socketPath string) *UDSClient {
	return &UDSClient{
		socketPath: socketPath,
	}
}

// ensureClient lazily initializes the HTTP client.
func (c *UDSClient) ensureClient() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		return
	}

	c.client = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", c.socketPath)
			},
		},
	}
}

// SendActionInvocationEndpointRequest implements faroe's ActionInvocationEndpointClientInterface
// by sending JSON requests to Python's /invoke endpoint over UDS.
func (c *UDSClient) SendActionInvocationEndpointRequest(requestJSON string) (string, error) {
	c.ensureClient()

	// Use "http://uds" as a placeholder - the actual connection goes through the UDS
	req, err := http.NewRequest("POST", "http://uds/invoke", bytes.NewReader([]byte(requestJSON)))
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
func (c *UDSClient) SendTestNotification(action, email, code string) error {
	c.ensureClient()

	payload := map[string]string{
		"action": action,
		"email":  email,
		"code":   code,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	req, err := http.NewRequest("POST", "http://uds/token", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token notification failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
