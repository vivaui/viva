package app

import "fmt"

// StoreSecretAsync is StoreSecret with a completion callback, the way to save
// a secret on the Web target, where encryption through WebCrypto settles
// through Promises. On native targets done runs before the call returns, with
// the same result as StoreSecret. done runs exactly once.
//
// A browser keeps secrets in the page's own origin storage, encrypted with a
// non-extractable AES-GCM key held by IndexedDB. That protects the bytes at
// rest; it cannot protect them from script running in the same origin, so a
// page that loads untrusted third-party script must not keep credentials this
// way.
func StoreSecretAsync(service, account string, secret []byte, done func(error)) error {
	if done == nil {
		return fmt.Errorf("app: nil secret callback")
	}
	if err := validateSecretKey(service, account); err != nil {
		return err
	}
	if err := validateSecretValue(secret); err != nil {
		return err
	}
	return asyncStoreSecret(service, account, secret, done)
}

// LoadSecretAsync is LoadSecret with a completion callback. done runs exactly
// once, with ErrSecretNotFound when nothing is stored.
func LoadSecretAsync(service, account string, done func([]byte, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil secret callback")
	}
	if err := validateSecretKey(service, account); err != nil {
		return err
	}
	return asyncLoadSecret(service, account, done)
}

// DeleteSecretAsync is DeleteSecret with a completion callback. Removing what
// is not there is not an error. done runs exactly once.
func DeleteSecretAsync(service, account string, done func(error)) error {
	if done == nil {
		return fmt.Errorf("app: nil secret callback")
	}
	if err := validateSecretKey(service, account); err != nil {
		return err
	}
	return asyncDeleteSecret(service, account, done)
}
