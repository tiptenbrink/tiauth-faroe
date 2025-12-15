package tiauth

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type userActionInvocationClientStruct struct {
	endpoint              string
	privateRouteAccessKey string
}

func newUserActionInvocationClient(endpoint string, privateRouteAccessKey string) *userActionInvocationClientStruct {
	return &userActionInvocationClientStruct{
		endpoint:              endpoint,
		privateRouteAccessKey: privateRouteAccessKey,
	}
}

func (userActionInvocationClient *userActionInvocationClientStruct) SendActionInvocationEndpointRequest(body string) (string, error) {
	request, _ := http.NewRequest("POST", userActionInvocationClient.endpoint, strings.NewReader(body))

	request.Header.Set("Content-Type", "application/json")
	if userActionInvocationClient.privateRouteAccessKey != "" {
		request.Header.Set("x-private-route-access-key", userActionInvocationClient.privateRouteAccessKey)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %s", err.Error())
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return "", fmt.Errorf("unexpected status code %d", response.StatusCode)
	}
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %s", err.Error())
	}
	return string(bodyBytes), nil
}
