//go:build darwin

package proxy

import (
	"errors"
	"fmt"
	"net/url"

	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

func nativeSystemProxy(target *url.URL) (*url.URL, error) {
	proxy, err := darwinplatform.SystemProxy(target.String())
	if errors.Is(err, darwinplatform.ErrProxyUnavailable) {
		return nil, fmt.Errorf("%w: %v", errSystemProxyUnavailable, err)
	}
	return proxy, err
}
