//go:build darwin && !ios

package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

func nativeSystemProxy(ctx context.Context, target *url.URL) (*url.URL, error) {
	proxy, err := darwinplatform.SystemProxyContext(ctx, target.String())
	if errors.Is(err, darwinplatform.ErrProxyUnavailable) {
		return nil, fmt.Errorf("%w: %v", errSystemProxyUnavailable, err)
	}
	return proxy, err
}
