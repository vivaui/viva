//go:build windows

package proxy

import (
	"errors"
	"fmt"
	"net/url"

	windowsplatform "github.com/vivaui/viva/internal/platform/windows"
)

func nativeSystemProxy(target *url.URL) (*url.URL, error) {
	proxy, err := windowsplatform.SystemProxy(target.String())
	if errors.Is(err, windowsplatform.ErrProxyUnavailable) {
		return nil, fmt.Errorf("%w: %v", errSystemProxyUnavailable, err)
	}
	return proxy, err
}
