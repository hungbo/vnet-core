package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func httpPost(url string, body interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	log.Printf("[HTTP] POST %s body=%s", url, string(mustMarshal(body)))

	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		log.Printf("[HTTP] new request error: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[HTTP] do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[HTTP] POST %s → %d body=%s", url, resp.StatusCode, string(bodyBytes))
	return nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
