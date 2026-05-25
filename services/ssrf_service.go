package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/url"
	"strings"
	"time"
)

func ValidateNoPrivateURLs(body []byte, contentType string) error {
	if OptionServiceApp.Int64("relay.ssrf_check_enabled", 1) <= 0 {
		return nil
	}
	urls := extractRequestURLs(body, contentType)
	for _, rawURL := range urls {
		if err := validatePublicURL(rawURL); err != nil {
			return err
		}
	}
	return nil
}

func extractRequestURLs(body []byte, contentType string) []string {
	if strings.Contains(contentType, "multipart/form-data") {
		return extractMultipartURLs(body, contentType)
	}
	if strings.Contains(contentType, "application/json") || contentType == "" {
		var payload any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil
		}
		out := []string{}
		collectJSONURLs(payload, "", &out)
		return out
	}
	return nil
}

func extractMultipartURLs(body []byte, contentType string) []string {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return nil
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return nil
	}
	defer form.RemoveAll()
	out := []string{}
	for key, values := range form.Value {
		if !isURLField(key) {
			continue
		}
		for _, value := range values {
			if looksLikeHTTPURL(value) {
				out = append(out, value)
			}
		}
	}
	return out
}

func collectJSONURLs(value any, key string, out *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			collectJSONURLs(childValue, childKey, out)
		}
	case []any:
		for _, item := range typed {
			collectJSONURLs(item, key, out)
		}
	case string:
		if isURLField(key) && looksLikeHTTPURL(typed) {
			*out = append(*out, typed)
		}
	}
}

func isURLField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "url" || strings.HasSuffix(key, "_url") || strings.Contains(key, "image_url") || strings.Contains(key, "audio_url") || strings.Contains(key, "file_url")
}

func looksLikeHTTPURL(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func validatePublicURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil
	}
	host := parsed.Hostname()
	if host == "" {
		return errors.New("url host is required")
	}
	if isLocalhostName(host) {
		return fmt.Errorf("private url is not allowed: %s", rawURL)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("private url is not allowed: %s", rawURL)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("url host cannot be resolved: %s", host)
	}
	for _, addr := range addrs {
		if isPrivateIP(addr.IP) {
			return fmt.Errorf("private url is not allowed: %s", rawURL)
		}
	}
	return nil
}

func isLocalhostName(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
