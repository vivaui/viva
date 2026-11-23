//go:build windows

package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	windowsplatform "github.com/vivaui/viva/internal/platform/windows"
)

func nativeSystemProxy(_ context.Context, target *url.URL) (*url.URL, error) {
	proxy, err := windowsplatform.SystemProxy(target.String())
	if errors.Is(err, windowsplatform.ErrProxyUnavailable) {
		return nil, fmt.Errorf("%w: %v", errSystemProxyUnavailable, err)
	}
	return proxy, err
}
