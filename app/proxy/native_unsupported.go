//go:build (!darwin || ios) && !windows && !linux && !(js && wasm)

package proxy

import (
	"context"
	"net/url"
)

func nativeSystemProxy(context.Context, *url.URL) (*url.URL, error) { return nil, ErrUnavailable }
