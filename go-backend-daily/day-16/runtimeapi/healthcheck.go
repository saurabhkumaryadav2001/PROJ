package main

import (
	"fmt"
	"net/http"
	"time"
)

// checkHealth is separated from the CLI wrapper so it can be tested with
// httptest.NewServer without binding a fixed port.
func checkHealth(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}

func healthcheckURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
}
