package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func GetStringPtr(value interface{}) *string {
	if value == nil {
		return nil
	}
	strValue, ok := value.(string)
	if !ok {
		return nil
	}
	return &strValue
}

func GetIntPtr(value interface{}) *int {
	if value == nil {
		return nil
	}
	floatValue, ok := value.(float64)
	if !ok {
		return nil
	}
	intValue := int(floatValue)
	return &intValue
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func DoRequest(client HTTPClient, url string, headers map[string]string, timeoutSecond int) ([]byte, int, error) {
	if timeoutSecond == 0 {
		timeoutSecond = 5
	}
	timeout := time.Duration(timeoutSecond) * time.Second

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %v", err)
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	clientWithTimeout := &http.Client{
		Timeout: timeout,
	}

	res, err := clientWithTimeout.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, fmt.Errorf("failed to read response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return body, res.StatusCode, fmt.Errorf("failed to get response, status code: %d", res.StatusCode)
	}

	return body, res.StatusCode, nil
}

// ParseJSONResponse parses the JSON response into the given result structure.
func ParseJSONResponse(body []byte, result interface{}) error {
	if !json.Valid(body) {
		return fmt.Errorf("invalid JSON response: %s", string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	return nil
}
