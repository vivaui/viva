//go:build js && wasm

package web

import (
	"errors"
	"fmt"
	"syscall/js"
)

var (
	// ErrSecretNotFound reports that the origin's secret store holds no entry
	// for the service and account.
	ErrSecretNotFound = errors.New("web: secret not found")
)

const (
	secretDatabase = "viva-secrets"
	secretStore    = "secrets"
	secretKeyID    = "__viva_master_key__"
	secretVersion  = 1
)

// StoreSecretAsync encrypts secret with a non-extractable AES-GCM key held in
// IndexedDB and stores the ciphertext under service and account. The key never
// leaves the browser; an XSS in the same origin can still ask WebCrypto to use
// it, which is why the documentation says not to keep credentials on a page
// that runs untrusted script.
func StoreSecretAsync(service, account string, secret []byte, done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil secret callback", ErrUnavailable)
	}
	runSecretOperation(func(finish func()) {
		withSecretDatabase(func(db js.Value, err error) {
			if err != nil {
				finish()
				done(err)
				return
			}
			withSecretKey(db, func(key js.Value, err error) {
				if err != nil {
					db.Call("close")
					finish()
					done(err)
					return
				}
				sealSecret(db, key, service, account, secret, func(err error) {
					db.Call("close")
					finish()
					done(err)
				})
			})
		})
	})
	return nil
}

// LoadSecretAsync decrypts the entry for service and account.
func LoadSecretAsync(service, account string, done func([]byte, error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil secret callback", ErrUnavailable)
	}
	runSecretOperation(func(finish func()) {
		withSecretDatabase(func(db js.Value, err error) {
			if err != nil {
				finish()
				done(nil, err)
				return
			}
			databaseGet(db, secretEntry(service, account), func(record js.Value, err error) {
				if err != nil {
					db.Call("close")
					finish()
					done(nil, err)
					return
				}
				if record.Type() == js.TypeUndefined || record.IsNull() {
					db.Call("close")
					finish()
					done(nil, ErrSecretNotFound)
					return
				}
				withSecretKey(db, func(key js.Value, err error) {
					if err != nil {
						db.Call("close")
						finish()
						done(nil, err)
						return
					}
					openSecret(key, service, account, record, func(secret []byte, err error) {
						db.Call("close")
						finish()
						done(secret, err)
					})
				})
			})
		})
	})
	return nil
}

// DeleteSecretAsync removes the entry for service and account. Removing what
// is not there is not an error.
func DeleteSecretAsync(service, account string, done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil secret callback", ErrUnavailable)
	}
	runSecretOperation(func(finish func()) {
		withSecretDatabase(func(db js.Value, err error) {
			if err != nil {
				finish()
				done(err)
				return
			}
			databaseDelete(db, secretEntry(service, account), func(err error) {
				db.Call("close")
				finish()
				done(err)
			})
		})
	})
	return nil
}

var secretGate struct {
	busy    bool
	waiting []func()
}

// runSecretOperation serializes operations so two first writes cannot each
// generate a different master key.
func runSecretOperation(start func(finish func())) {
	run := func() {
		secretGate.busy = true
		finished := false
		start(func() {
			if finished {
				return
			}
			finished = true
			secretGate.busy = false
			if len(secretGate.waiting) > 0 {
				next := secretGate.waiting[0]
				secretGate.waiting = secretGate.waiting[1:]
				next()
			}
		})
	}
	if secretGate.busy {
		secretGate.waiting = append(secretGate.waiting, run)
		return
	}
	run()
}

func secretEntry(service, account string) string {
	return "secret\x00" + service + "\x00" + account
}

func secretAssociatedData(service, account string) string {
	return "viva-secret\x00" + service + "\x00" + account
}

func withSecretDatabase(done func(js.Value, error)) {
	indexedDB := js.Global().Get("indexedDB")
	if indexedDB.Type() != js.TypeObject || indexedDB.IsNull() {
		done(js.Undefined(), fmt.Errorf("%w: indexedDB is missing", ErrUnavailable))
		return
	}
	var request js.Value
	if err := catchJS("open secret database", func() { request = indexedDB.Call("open", secretDatabase, secretVersion) }); err != nil {
		done(js.Undefined(), err)
		return
	}
	var upgrade, success, failure js.Func
	finished := false
	release := func() {
		for _, function := range []js.Func{upgrade, success, failure} {
			if function.Type() == js.TypeFunction {
				function.Release()
			}
		}
	}
	finish := func(db js.Value, err error) {
		if finished {
			return
		}
		finished = true
		release()
		done(db, err)
	}
	upgrade = js.FuncOf(func(js.Value, []js.Value) any {
		db := request.Get("result")
		if !db.Get("objectStoreNames").Call("contains", secretStore).Bool() {
			db.Call("createObjectStore", secretStore)
		}
		return nil
	})
	success = js.FuncOf(func(js.Value, []js.Value) any {
		finish(request.Get("result"), nil)
		return nil
	})
	failure = js.FuncOf(func(js.Value, []js.Value) any {
		finish(js.Undefined(), databaseError(request.Get("error")))
		return nil
	})
	request.Set("onupgradeneeded", upgrade)
	request.Set("onsuccess", success)
	request.Set("onerror", failure)
}

func withSecretKey(db js.Value, done func(js.Value, error)) {
	databaseGet(db, secretKeyID, func(value js.Value, err error) {
		if err != nil {
			done(js.Undefined(), err)
			return
		}
		if value.Type() != js.TypeUndefined && !value.IsNull() {
			done(value, nil)
			return
		}
		crypto := js.Global().Get("crypto")
		subtle := crypto.Get("subtle")
		if subtle.Type() != js.TypeObject || subtle.IsNull() {
			done(js.Undefined(), fmt.Errorf("%w: WebCrypto is unavailable (a secure context is required)", ErrUnavailable))
			return
		}
		algorithm := object()
		algorithm.Set("name", "AES-GCM")
		algorithm.Set("length", 256)
		usages := js.Global().Get("Array").New()
		usages.Call("push", "encrypt")
		usages.Call("push", "decrypt")
		var promise js.Value
		if err := catchJS("generate secret key", func() {
			promise = subtle.Call("generateKey", algorithm, false, usages)
		}); err != nil {
			done(js.Undefined(), err)
			return
		}
		if err := awaitPromise(promise, func(key js.Value) {
			databasePut(db, secretKeyID, key, func(err error) {
				if err != nil {
					done(js.Undefined(), err)
					return
				}
				done(key, nil)
			})
		}, func(err error) { done(js.Undefined(), secretError(err)) }); err != nil {
			done(js.Undefined(), err)
		}
	})
}

func sealSecret(db, key js.Value, service, account string, secret []byte, done func(error)) {
	crypto := js.Global().Get("crypto")
	subtle := crypto.Get("subtle")
	if subtle.Type() != js.TypeObject || subtle.IsNull() {
		done(fmt.Errorf("%w: WebCrypto is unavailable", ErrUnavailable))
		return
	}
	iv := crypto.Call("getRandomValues", js.Global().Get("Uint8Array").New(12))
	plaintext := js.Global().Get("Uint8Array").New(len(secret))
	js.CopyBytesToJS(plaintext, secret)
	algorithm := object()
	algorithm.Set("name", "AES-GCM")
	algorithm.Set("iv", iv)
	algorithm.Set("additionalData", secretAAD(service, account))
	var promise js.Value
	if err := catchJS("encrypt secret", func() {
		promise = subtle.Call("encrypt", algorithm, key, plaintext)
	}); err != nil {
		done(err)
		return
	}
	if err := awaitPromise(promise, func(ciphertext js.Value) {
		record := object()
		record.Set("iv", iv)
		record.Set("ciphertext", ciphertext)
		databasePut(db, secretEntry(service, account), record, func(err error) {
			if err != nil {
				done(err)
				return
			}
			done(nil)
		})
	}, func(err error) { done(secretError(err)) }); err != nil {
		done(err)
	}
}

func openSecret(key js.Value, service, account string, record js.Value, done func([]byte, error)) {
	crypto := js.Global().Get("crypto")
	subtle := crypto.Get("subtle")
	if subtle.Type() != js.TypeObject || subtle.IsNull() {
		done(nil, fmt.Errorf("%w: WebCrypto is unavailable", ErrUnavailable))
		return
	}
	algorithm := object()
	algorithm.Set("name", "AES-GCM")
	algorithm.Set("iv", record.Get("iv"))
	algorithm.Set("additionalData", secretAAD(service, account))
	var promise js.Value
	if err := catchJS("decrypt secret", func() {
		promise = subtle.Call("decrypt", algorithm, key, record.Get("ciphertext"))
	}); err != nil {
		done(nil, err)
		return
	}
	if err := awaitPromise(promise, func(buffer js.Value) {
		size := buffer.Get("byteLength").Int()
		secret := make([]byte, size)
		if copied := js.CopyBytesToGo(secret, js.Global().Get("Uint8Array").New(buffer)); copied != size {
			done(nil, fmt.Errorf("web: copy secret bytes: copied %d of %d", copied, size))
			return
		}
		done(secret, nil)
	}, func(err error) { done(nil, secretError(err)) }); err != nil {
		done(nil, err)
	}
}

func secretAAD(service, account string) js.Value {
	encoder := js.Global().Get("TextEncoder")
	if encoder.Type() != js.TypeFunction {
		return js.Global().Get("Uint8Array").New(0)
	}
	return encoder.New().Call("encode", secretAssociatedData(service, account))
}

func databaseGet(db js.Value, key string, done func(js.Value, error)) {
	var transaction, request js.Value
	if err := catchJS("read secret", func() {
		transaction = db.Call("transaction", secretStore, "readonly")
		request = transaction.Call("objectStore", secretStore).Call("get", key)
	}); err != nil {
		done(js.Undefined(), err)
		return
	}
	var success, failure js.Func
	finished := false
	release := func() {
		success.Release()
		failure.Release()
	}
	finish := func(value js.Value, err error) {
		if finished {
			return
		}
		finished = true
		release()
		done(value, err)
	}
	success = js.FuncOf(func(js.Value, []js.Value) any {
		finish(request.Get("result"), nil)
		return nil
	})
	failure = js.FuncOf(func(js.Value, []js.Value) any {
		finish(js.Undefined(), databaseError(request.Get("error")))
		return nil
	})
	request.Set("onsuccess", success)
	request.Set("onerror", failure)
}

func databasePut(db js.Value, key string, value js.Value, done func(error)) {
	databaseWrite(db, "put", key, value, done)
}

func databaseDelete(db js.Value, key string, done func(error)) {
	databaseWrite(db, "delete", key, js.Undefined(), done)
}

func databaseWrite(db js.Value, method, key string, value js.Value, done func(error)) {
	var transaction, request js.Value
	if err := catchJS("write secret", func() {
		transaction = db.Call("transaction", secretStore, "readwrite")
		store := transaction.Call("objectStore", secretStore)
		if method == "put" {
			request = store.Call("put", value, key)
		} else {
			request = store.Call("delete", key)
		}
	}); err != nil {
		done(err)
		return
	}
	var complete, failure, abort js.Func
	finished := false
	release := func() {
		for _, function := range []js.Func{complete, failure, abort} {
			function.Release()
		}
	}
	finish := func(err error) {
		if finished {
			return
		}
		finished = true
		release()
		done(err)
	}
	complete = js.FuncOf(func(js.Value, []js.Value) any {
		// Wait for the transaction to commit, not only the request to succeed.
		finish(nil)
		return nil
	})
	failure = js.FuncOf(func(js.Value, []js.Value) any {
		finish(databaseError(request.Get("error")))
		return nil
	})
	abort = js.FuncOf(func(js.Value, []js.Value) any {
		finish(databaseError(transaction.Get("error")))
		return nil
	})
	_ = request
	transaction.Set("oncomplete", complete)
	transaction.Set("onerror", failure)
	transaction.Set("onabort", abort)
}

func databaseError(value js.Value) error {
	if value.Type() == js.TypeObject && !value.IsNull() {
		switch value.Get("name").String() {
		case "SecurityError", "NotAllowedError":
			return fmt.Errorf("%w: %s", ErrPermissionDenied, value.Get("message").String())
		case "QuotaExceededError":
			return fmt.Errorf("%w: %s", ErrUnavailable, value.Get("message").String())
		}
		if message := value.Get("message").String(); message != "" {
			return fmt.Errorf("%w: %s", ErrUnavailable, message)
		}
	}
	return fmt.Errorf("%w: the secret database rejected the operation", ErrUnavailable)
}

func secretError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
