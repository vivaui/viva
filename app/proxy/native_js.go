//go:build js && wasm

package proxy

import (
	"context"
	"net/url"
)

func nativeSystemProxy(context.Context, *url.URL) (*url.URL, error) { return nil, ErrUnavailable }
