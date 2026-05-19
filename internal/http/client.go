package http

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hieuhm2/lazyapi/internal/storage"
)

func Execute(req storage.Request) storage.Response {
	start := time.Now()

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return storage.Response{Error: err.Error()}
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if req.Body != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return storage.Response{Error: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return storage.Response{Error: err.Error()}
	}

	elapsed := time.Since(start)

	return storage.Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       string(body),
		DurationMs: elapsed.Milliseconds(),
		SizeBytes:  int64(len(body)),
	}
}
