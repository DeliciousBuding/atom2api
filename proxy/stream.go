package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func PipeSSE(w http.ResponseWriter, src io.Reader) {
	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			fmt.Fprintf(w, "\n")
			if canFlush {
				flusher.Flush()
			}
			continue
		}
		fmt.Fprintf(w, "%s\n", line)
		if canFlush {
			flusher.Flush()
		}
		if strings.HasPrefix(line, "data: [DONE]") {
			break
		}
	}
}

func ExtractUsageFromBody(body []byte) (promptTokens, completionTokens, totalTokens int) {
	// Simple extraction: look for "usage" in the response body
	// This is a best-effort parse for logging
	s := string(body)
	if idx := strings.Index(s, `"prompt_tokens"`); idx > 0 {
		fmt.Sscanf(s[idx:], `"prompt_tokens":%d`, &promptTokens)
	}
	if idx := strings.Index(s, `"completion_tokens"`); idx > 0 {
		fmt.Sscanf(s[idx:], `"completion_tokens":%d`, &completionTokens)
	}
	if idx := strings.Index(s, `"total_tokens"`); idx > 0 {
		fmt.Sscanf(s[idx:], `"total_tokens":%d`, &totalTokens)
	}
	return
}
