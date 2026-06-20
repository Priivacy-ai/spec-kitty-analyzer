package analyzer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxFlattenJSONStringBytes = 4096

func flattenJSON(obj map[string]any) string {
	var parts []string
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case string:
			if len(typed) > maxFlattenJSONStringBytes {
				typed = typed[:maxFlattenJSONStringBytes] + " [truncated]"
			}
			parts = append(parts, typed)
		case float64, bool:
			parts = append(parts, fmt.Sprint(typed))
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(typed[key])
			}
		}
	}
	walk(obj)
	return strings.Join(parts, " ")
}

func jsonKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func firstJSONStringByKey(value any, targets ...string) string {
	target := map[string]bool{}
	for _, t := range targets {
		target[t] = true
	}
	var found string
	var walk func(any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if target[key] {
					if text, ok := item.(string); ok {
						found = text
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func firstJSONNumberByKey(value any, targets ...string) (float64, bool) {
	target := map[string]bool{}
	for _, t := range targets {
		target[t] = true
	}
	var found float64
	ok := false
	var walk func(any)
	walk = func(v any) {
		if ok {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if target[key] {
					switch n := item.(type) {
					case float64:
						found, ok = n, true
						return
					case int:
						found, ok = float64(n), true
						return
					case string:
						if parsed, err := strconv.ParseFloat(n, 64); err == nil {
							found, ok = parsed, true
							return
						}
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found, ok
}

func firstJSONMapByKey(value any, targets ...string) map[string]any {
	target := map[string]bool{}
	for _, t := range targets {
		target[t] = true
	}
	var found map[string]any
	var walk func(any)
	walk = func(v any) {
		if found != nil {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if target[key] {
					if m, ok := item.(map[string]any); ok {
						found = m
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func jsonHasAnyKey(value any, targets ...string) bool {
	target := map[string]bool{}
	for _, t := range targets {
		target[t] = true
	}
	found := false
	var walk func(any)
	walk = func(v any) {
		if found {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if target[key] {
					found = true
					return
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func jsonHasError(obj map[string]any) bool {
	if text := firstJSONStringByKey(obj, "error", "exception", "traceback"); strings.TrimSpace(text) != "" {
		return true
	}
	if n, ok := firstJSONNumberByKey(obj, "exit_code", "returncode", "return_code"); ok && n != 0 {
		return true
	}
	status := strings.ToLower(firstJSONStringByKey(obj, "status", "outcome", "kind", "verdict"))
	return status == "failed" || status == "failure" || status == "blocked" || status == "rejected" || status == "error"
}

func jsonLooksLikeSourceRead(obj map[string]any) bool {
	result, ok := obj["toolUseResult"].(map[string]any)
	if !ok {
		return false
	}
	file, ok := result["file"].(map[string]any)
	if !ok {
		return false
	}
	_, hasContent := file["content"].(string)
	_, hasPath := file["filePath"].(string)
	return hasContent && hasPath
}

func parseJSONTime(obj map[string]any) *time.Time {
	raw := firstJSONStringByKey(obj, "timestamp", "created_at", "updated_at", "started_at", "completed_at", "time", "ts")
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}

func decodeJSONObject(data []byte) (map[string]any, bool) {
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return nil, false
	}
	return obj, true
}
