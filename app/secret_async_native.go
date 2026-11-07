//go:build !(js && wasm)

package app

func asyncStoreSecret(service, account string, secret []byte, done func(error)) error {
	done(StoreSecret(service, account, secret))
	return nil
}

func asyncLoadSecret(service, account string, done func([]byte, error)) error {
	secret, err := LoadSecret(service, account)
	done(secret, err)
	return nil
}

func asyncDeleteSecret(service, account string, done func(error)) error {
	done(DeleteSecret(service, account))
	return nil
}
