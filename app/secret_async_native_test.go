//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"
)

func TestAsyncSecretUsesTheSameStoreAndValidatesOnce(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	service, account := "viva-async-test", "key"
	called := 0
	if err := StoreSecretAsync(service, account, []byte("secret"), func(err error) {
		called++
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("StoreSecretAsync called done %d times", called)
	}
	var loaded []byte
	if err := LoadSecretAsync(service, account, func(secret []byte, err error) {
		loaded = secret
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if string(loaded) != "secret" {
		t.Fatalf("loaded = %q", loaded)
	}
	if err := DeleteSecretAsync(service, account, func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := LoadSecretAsync(service, account, func([]byte, error) {}); err != nil {
		t.Fatal(err)
	}
	var notFound error
	if err := LoadSecretAsync(service, account, func(_ []byte, err error) { notFound = err }); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(notFound, ErrSecretNotFound) {
		t.Fatalf("after delete = %v", notFound)
	}
	if err := StoreSecretAsync(service, account, nil, func(error) {}); err == nil {
		t.Fatal("StoreSecretAsync accepted an empty secret")
	}
}
