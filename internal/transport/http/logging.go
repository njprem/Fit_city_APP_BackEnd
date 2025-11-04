package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

const (
	requestBodyLogKey  = "http.request.body.summary"
	responseBodyLogKey = "http.response.body.summary"
	maxLoggedBody      = 2048
)

func registerLogging(e *echo.Echo) {
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:      true,
		LogStatus:   true,
		LogMethod:   true,
		LogLatency:  true,
		LogError:    true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			userID := "anonymous"
			if user, ok := c.Get(contextUserKey).(*domain.User); ok && user != nil {
				userID = user.ID.String()
			}

			reqBodySummary := c.Get(requestBodyLogKey)
			resBodySummary := c.Get(responseBodyLogKey)

			payload := struct {
				Time      string `json:"time"`
				UserUUID  string `json:"user_uuid"`
				LatencyMS int64  `json:"latency_ms"`
				Request   struct {
					Method string      `json:"method"`
					URI    string      `json:"uri"`
					Body   interface{} `json:"body,omitempty"`
				} `json:"request"`
				Response struct {
					Status int         `json:"status"`
					Body   interface{} `json:"body,omitempty"`
					Error  string      `json:"error,omitempty"`
				} `json:"response"`
			}{
				Time:      v.StartTime.Format(time.RFC3339),
				UserUUID:  userID,
				LatencyMS: v.Latency.Milliseconds(),
			}

			payload.Request.Method = v.Method
			payload.Request.URI = v.URI
			if reqBodySummary != nil {
				payload.Request.Body = reqBodySummary
			}

			payload.Response.Status = v.Status
			if resBodySummary != nil {
				payload.Response.Body = resBodySummary
			}
			if v.Error != nil {
				payload.Response.Error = v.Error.Error()
			}

			buf, err := json.Marshal(payload)
			if err != nil {
				return err
			}

			log.Println(string(buf))
			return nil
		},
	}))

	e.Use(middleware.BodyDump(func(c echo.Context, reqBody, resBody []byte) {
		if summary := sanitizeBody(reqBody, c.Request().Header.Get(echo.HeaderContentType)); summary != nil {
			c.Set(requestBodyLogKey, summary)
		}
		if summary := sanitizeBody(resBody, c.Response().Header().Get(echo.HeaderContentType)); summary != nil {
			c.Set(responseBodyLogKey, summary)
		}
	}))
}

func sanitizeBody(body []byte, contentType string) interface{} {
	if len(body) == 0 {
		return nil
	}

	trimmedType := strings.TrimSpace(contentType)
	loweredType := strings.ToLower(trimmedType)

	if strings.HasPrefix(loweredType, "multipart/form-data") {
		return sanitizeMultipart(body, trimmedType)
	}

	isJSON := strings.HasPrefix(loweredType, "application/json") || json.Valid(body)
	if isJSON {
		var data interface{}
		if err := json.Unmarshal(body, &data); err == nil {
			sanitized := sanitizeJSON(data, "")
			return limitJSONSize(sanitized)
		}
	}

	if strings.HasPrefix(loweredType, "application/x-www-form-urlencoded") {
		if values, err := url.ParseQuery(string(body)); err == nil {
			sanitized := make(map[string]interface{}, len(values))
			for key, vals := range values {
				lowerKey := strings.ToLower(key)
				if strings.Contains(lowerKey, "password") {
					sanitized[key] = "redacted"
					continue
				}
				slice := make([]interface{}, 0, len(vals))
				for _, v := range vals {
					slice = append(slice, sanitizeStringValue(v, lowerKey))
				}
				if len(slice) == 1 {
					sanitized[key] = slice[0]
				} else {
					sanitized[key] = slice
				}
			}
			if len(sanitized) > 0 {
				return limitJSONSize(sanitized)
			}
		}
	}

	if containsBinaryBytes(body) {
		return "binary"
	}

	text := string(body)
	if strings.Contains(strings.ToLower(text), "password") {
		return "redacted"
	}

	return clampString(text)
}

func limitJSONSize(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	buf, err := json.Marshal(value)
	if err != nil {
		return value
	}
	if len(buf) <= maxLoggedBody {
		return value
	}
	summary := summarizeJSONPreview(value, 0)
	if summary == nil {
		return map[string]interface{}{"_truncated": true}
	}
	return map[string]interface{}{
		"_truncated": true,
		"_preview":   summary,
	}
}

func sanitizeJSON(value interface{}, keyHint string) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "password") {
				result[key] = "redacted"
				continue
			}
			result[key] = sanitizeJSON(val, lowerKey)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = sanitizeJSON(item, keyHint)
		}
		return result
	case string:
		return sanitizeStringValue(v, keyHint)
	default:
		return v
	}
}

func sanitizeStringValue(value string, keyHint string) string {
	if keyHint != "" && strings.Contains(keyHint, "password") {
		return "redacted"
	}
	if containsBinaryBytes([]byte(value)) {
		return "binary"
	}
	return clampString(value)
}

func sanitizeMultipart(body []byte, contentType string) interface{} {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "binary"
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		return "binary"
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "binary"
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	fields := make(map[string]interface{})

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "binary"
		}

		name := part.FormName()
		if name == "" {
			_ = part.Close()
			continue
		}

		lowerName := strings.ToLower(name)
		var value interface{}

		if part.FileName() != "" {
			value = "binary"
		} else {
			data, err := io.ReadAll(part)
			if err != nil {
				value = "binary"
			} else {
				value = sanitizeStringValue(string(data), lowerName)
			}
		}
		_ = part.Close()
		addFormField(fields, name, value)
	}

	if len(fields) == 0 {
		return "binary"
	}

	return limitJSONSize(fields)
}

func summarizeJSONPreview(value interface{}, depth int) interface{} {
	const (
		maxDepth          = 3
		maxMapEntries     = 6
		maxArraySamples   = 3
		maxStringPreview  = 256
		omittedKeysLabel  = "_omitted_fields"
		omittedItemsLabel = "_omitted_items"
		arrayLenLabel     = "_total_items"
		sampleLabel       = "_sample"
	)

	if depth >= maxDepth {
		return "...(omitted)..."
	}

	switch v := value.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return map[string]interface{}{}
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		result := make(map[string]interface{})
		count := 0
		for _, key := range keys {
			if count >= maxMapEntries {
				result[omittedKeysLabel] = len(keys) - count
				break
			}
			result[key] = summarizeJSONPreview(v[key], depth+1)
			count++
		}
		if len(keys) > count {
			result[omittedKeysLabel] = len(keys) - count
		}
		return result
	case []interface{}:
		total := len(v)
		if total == 0 {
			return []interface{}{}
		}
		sample := make([]interface{}, 0, minInt(total, maxArraySamples))
		for i := 0; i < total && i < maxArraySamples; i++ {
			sample = append(sample, summarizeJSONPreview(v[i], depth+1))
		}
		out := map[string]interface{}{
			arrayLenLabel: total,
		}
		if len(sample) > 0 {
			out[sampleLabel] = sample
		}
		if total > len(sample) {
			out[omittedItemsLabel] = total - len(sample)
		}
		return out
	case string:
		if len(v) <= maxStringPreview {
			return v
		}
		preview := v[:maxStringPreview]
		for !utf8.ValidString(preview) && len(preview) > 0 {
			preview = preview[:len(preview)-1]
		}
		return preview + "...(truncated)"
	default:
		return v
	}
}

func containsBinaryBytes(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return true
		}
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return true
		}
		data = data[size:]
	}
	return false
}

func clampString(value string) string {
	if len(value) <= maxLoggedBody {
		return value
	}
	truncated := value[:maxLoggedBody]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "...(truncated)"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func addFormField(fields map[string]interface{}, key string, value interface{}) {
	if existing, ok := fields[key]; ok {
		switch items := existing.(type) {
		case []interface{}:
			fields[key] = append(items, value)
		default:
			fields[key] = []interface{}{items, value}
		}
		return
	}
	fields[key] = value
}
