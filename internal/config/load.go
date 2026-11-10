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
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("配置地址为空")
	}
	var body []byte
	var source string
	var err error
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		body, _, err = httpx.Get(ctx, nil, ref, nil, maxDecodedSize)
		source = ref
	} else if strings.HasPrefix(ref, "file:") {
		body, _, err = httpx.Get(ctx, nil, ref, nil, maxDecodedSize)
		source = ref
	} else {
		body, err = os.ReadFile(ref)
		source = FileURL(ref)
	}
	if err != nil {
		return nil, err
	}
	cfg, err := Parse(body)
	if err != nil {
		return nil, err
	}
	cfg.SourceURL = source
	cfg.ResolveRelatives()
	return cfg, nil
}
