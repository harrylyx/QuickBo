package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/harrylyx/QuickBo/internal/httpx"
)

// Load reads a subscription from an http(s) URL or a local path.
func Load(ctx context.Context, ref string) (*Config, error) {
	body, source, err := Fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	return FromBytes(body, source)
}

// Fetch reads the raw subscription text behind ref, and returns it with the
// URL that relative addresses in it resolve against. Callers that keep a copy
// of the text turn it into a Config later with FromBytes.
func Fetch(ctx context.Context, ref string) (body []byte, source string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, "", fmt.Errorf("配置地址为空")
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "file:") {
		body, _, err = httpx.Get(ctx, nil, ref, nil, maxDecodedSize)
		return body, ref, err
	}
	body, err = os.ReadFile(ref)
	return body, FileURL(ref), err
}

// FromBytes parses subscription text fetched from source.
func FromBytes(body []byte, source string) (*Config, error) {
	cfg, err := Parse(body)
	if err != nil {
		return nil, err
	}
	cfg.SourceURL = source
	cfg.ResolveRelatives()
	return cfg, nil
}
