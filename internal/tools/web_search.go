package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// webSearchTool 提供可选 web 搜索能力；需配置 WEB_SEARCH_API_URL 环境变量。
func webSearchTool() Tool {
	return Tool{
		Name:        "web_search",
		Description: "Search the web for up-to-date information. Requires WEB_SEARCH_API_URL.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query",
				},
			},
			"required": []string{"query"},
		},
		Execute: runWebSearch,
	}
}

func runWebSearch(_ context.Context, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	apiURL := os.Getenv("WEB_SEARCH_API_URL")
	if apiURL == "" {
		return "", fmt.Errorf("web_search not configured: set WEB_SEARCH_API_URL")
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("invalid WEB_SEARCH_API_URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search api %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if json.Valid(body) {
		var v any
		if err := json.Unmarshal(body, &v); err == nil {
			enc, err := json.MarshalIndent(v, "", "  ")
			if err == nil {
				return string(enc), nil
			}
		}
	}
	return string(body), nil
}
