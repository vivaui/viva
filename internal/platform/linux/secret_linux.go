package linux

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

// The Secret Service (org.freedesktop.secrets) is what GNOME Keyring, KWallet,
// KeePassXC and others provide to the desktop. Items live in a collection, here
// the default one, and carry attributes that are searched for. The transport is
// the session bus; the "plain" session algorithm sends the secret unencrypted
// over it, which is the same trust the bus already has: a process that can read
// another's bus traffic can ask the store itself.
const (
	secretsName       = "org.freedesktop.secrets"
	secretsPath       = "/org/freedesktop/secrets"
	secretsService    = "org.freedesktop.Secret.Service"
	secretsCollection = "org.freedesktop.Secret.Collection"
	secretsItem       = "org.freedesktop.Secret.Item"
	secretsPrompt     = "org.freedesktop.Secret.Prompt"
	secretsDefault    = "/org/freedesktop/secrets/aliases/default"
	secretsNoObject   = "/"
	secretsItemLabel  = "org.freedesktop.Secret.Item.Label"
	secretsItemAttrs  = "org.freedesktop.Secret.Item.Attributes"
	secretsProperties = "org.freedesktop.DBus.Properties"

	// promptTimeout bounds how long the user is given to answer an unlock prompt.
	promptTimeout = 2 * time.Minute
)

// secretStore is one connection to the Secret Service and its open session.
type secretStore struct {
	conn    *dbus.Conn
	session string
}

// openSecretStore connects to the session bus and opens a session with the
// Secret Service. A bus without the service, or no bus, is "no store".
func openSecretStore() (*secretStore, error) {
	conn, err := dbus.DialSession()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", platform.ErrSecretStoreUnavailable, err)
	}
	store := &secretStore{conn: conn}
	var request dbus.Encoder
	request.String("plain")
	request.Variant("s")
	request.String("")
	reply, err := conn.Call(secretsName, secretsPath, secretsService, "OpenSession", "sv", request.Bytes())
	if err != nil {
		conn.Close()
		return nil, secretCallError("open a session", err)
	}
	args := reply.Args()
	args.Skip(args.Signature())
	store.session = args.String()
	if args.Err() != nil || store.session == "" {
		conn.Close()
		return nil, fmt.Errorf("linux: the Secret Service answered OpenSession badly")
	}
	return store, nil
}

func (store *secretStore) close() { store.conn.Close() }

// secretCallError names a failed call: a service that is not there means there
// is no store, one that is locked or refuses means denied.
func secretCallError(operation string, err error) error {
	var busError *dbus.Error
	if errors.As(err, &busError) {
		switch busError.Name {
		case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner",
			"org.freedesktop.DBus.Error.NoReply", "org.freedesktop.DBus.Error.Spawn.ServiceNotFound",
			"org.freedesktop.DBus.Error.Spawn.ExecFailed":
			return fmt.Errorf("%w: %s: %v", platform.ErrSecretStoreUnavailable, operation, err)
		case "org.freedesktop.Secret.Error.IsLocked", "org.freedesktop.DBus.Error.AccessDenied":
			return fmt.Errorf("%w: %s: %v", platform.ErrSecretDenied, operation, err)
		case "org.freedesktop.Secret.Error.NoSuchObject":
			return platform.ErrSecretNotFound
		}
	}
	return fmt.Errorf("linux: secret service %s: %w", operation, err)
}

// attributes is the pair of attributes that identify an item.
func secretAttributes(e *dbus.Encoder, service, account string) {
	mark := e.BeginArray(8)
	for _, pair := range [...][2]string{{"account", account}, {"service", service}} {
		e.Align(8)
		e.String(pair[0])
		e.String(pair[1])
	}
	e.EndArray(mark)
}

func readObjectPaths(args *dbus.Decoder) []string {
	end := args.ArrayLength(4)
	var paths []string
	for args.Pos() < end && args.Err() == nil {
		paths = append(paths, args.String())
	}
	return paths
}

// waitPrompt runs a prompt the service asked for and waits for it to finish. A
// prompt the user dismisses is a refusal.
func (store *secretStore) waitPrompt(prompt string) error {
	if prompt == secretsNoObject || prompt == "" {
		return nil
	}
	var match dbus.Encoder
	match.String("type='signal',interface='" + secretsPrompt + "',member='Completed'")
	if _, err := store.conn.Call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "AddMatch", "s", match.Bytes()); err != nil {
		return secretCallError("watch a prompt", err)
	}
	var window dbus.Encoder
	window.String("")
	if _, err := store.conn.Call(secretsName, prompt, secretsPrompt, "Prompt", "s", window.Bytes()); err != nil {
		return secretCallError("show a prompt", err)
	}
	deadline := time.Now().Add(promptTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// Block on the bus until something arrives: the user may take minutes.
		messages, err := store.conn.Wait(remaining)
		if err != nil {
			return fmt.Errorf("linux: secret service prompt: %w", err)
		}
		for _, message := range messages {
			if message.Type == dbus.TypeSignal && message.Member == "Completed" && message.Path == prompt {
				args := message.Args()
				if dismissed := args.Bool(); dismissed {
					return fmt.Errorf("%w: the prompt was dismissed", platform.ErrSecretDenied)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("%w: the prompt was not answered", platform.ErrSecretDenied)
}

// unlock asks the service to unlock the objects, prompting the user if need be.
func (store *secretStore) unlock(objects []string) error {
	var request dbus.Encoder
	mark := request.BeginArray(4)
	for _, object := range objects {
		request.String(object)
	}
	request.EndArray(mark)
	reply, err := store.conn.Call(secretsName, secretsPath, secretsService, "Unlock", "ao", request.Bytes())
	if err != nil {
		return secretCallError("unlock", err)
	}
	args := reply.Args()
	readObjectPaths(args)
	prompt := args.String()
	if args.Err() != nil {
		return args.Err()
	}
	return store.waitPrompt(prompt)
}

// defaultCollection is the collection secrets are kept in, unlocked.
func (store *secretStore) defaultCollection() (string, error) {
	locked, err := store.collectionLocked()
	if errors.Is(err, errNoDefaultCollection) {
		// A new account has no keyring yet. The service asks the user for its
		// password in a prompt of its own.
		if err := store.createDefaultCollection(); err != nil {
			return "", err
		}
		locked, err = store.collectionLocked()
	}
	if err != nil {
		return "", err
	}
	if locked {
		if err := store.unlock([]string{secretsDefault}); err != nil {
			return "", err
		}
	}
	return secretsDefault, nil
}

var errNoDefaultCollection = errors.New("linux: there is no default keyring")

// createDefaultCollection asks the service for the keyring applications keep
// their secrets in when the account has none.
func (store *secretStore) createDefaultCollection() error {
	var request dbus.Encoder
	properties := request.BeginArray(8)
	request.Align(8)
	request.String("org.freedesktop.Secret.Collection.Label")
	request.Variant("s")
	request.String("Default keyring")
	request.EndArray(properties)
	request.String("default")
	reply, err := store.conn.Call(secretsName, secretsPath, secretsService, "CreateCollection", "a{sv}s", request.Bytes())
	if err != nil {
		return secretCallError("create the default keyring", err)
	}
	args := reply.Args()
	_ = args.String() // the collection
	prompt := args.String()
	if args.Err() != nil {
		return args.Err()
	}
	return store.waitPrompt(prompt)
}

func (store *secretStore) collectionLocked() (bool, error) {
	var request dbus.Encoder
	request.String(secretsCollection)
	request.String("Locked")
	reply, err := store.conn.Call(secretsName, secretsDefault, secretsProperties, "Get", "ss", request.Bytes())
	if err != nil {
		var busError *dbus.Error
		if errors.As(err, &busError) && (busError.Name == "org.freedesktop.DBus.Error.UnknownObject" || busError.Name == "org.freedesktop.DBus.Error.UnknownMethod" ||
			busError.Name == "org.freedesktop.Secret.Error.NoSuchObject") {
			return false, errNoDefaultCollection
		}
		return false, secretCallError("read the collection", err)
	}
	args := reply.Args()
	signature := args.Signature()
	if signature != "b" {
		return false, fmt.Errorf("linux: the Secret Service reported Locked as %q", signature)
	}
	return args.Bool(), args.Err()
}

// find returns the items saved for the service and account, unlocking them when
// the service says they are locked.
func (store *secretStore) find(service, account string) ([]string, error) {
	search := func() (unlocked, locked []string, err error) {
		var request dbus.Encoder
		secretAttributes(&request, service, account)
		reply, err := store.conn.Call(secretsName, secretsPath, secretsService, "SearchItems", "a{ss}", request.Bytes())
		if err != nil {
			return nil, nil, secretCallError("search", err)
		}
		args := reply.Args()
		unlocked, locked = readObjectPaths(args), readObjectPaths(args)
		return unlocked, locked, args.Err()
	}
	unlocked, locked, err := search()
	if err != nil {
		return nil, err
	}
	if len(locked) > 0 {
		if err := store.unlock(locked); err != nil {
			return nil, err
		}
		if unlocked, _, err = search(); err != nil {
			return nil, err
		}
	}
	return unlocked, nil
}

// StoreSecret saves a secret in the default collection of the Secret Service,
// replacing one of the same service and account.
func StoreSecret(service, account string, secret []byte) error {
	store, err := openSecretStore()
	if err != nil {
		return err
	}
	defer store.close()
	collection, err := store.defaultCollection()
	if err != nil {
		return err
	}
	var request dbus.Encoder
	properties := request.BeginArray(8)
	request.Align(8)
	request.String(secretsItemLabel)
	request.Variant("s")
	request.String(service + ": " + account)
	request.Align(8)
	request.String(secretsItemAttrs)
	request.Variant("a{ss}")
	secretAttributes(&request, service, account)
	request.EndArray(properties)
	request.Align(8) // the secret: (session, parameters, value, content type)
	request.String(store.session)
	request.EndArray(request.BeginArray(1))
	value := request.BeginArray(1)
	for _, b := range secret {
		request.Byte(b)
	}
	request.EndArray(value)
	request.String("application/octet-stream")
	request.Bool(true) // replace the item of the same attributes
	reply, err := store.conn.Call(secretsName, collection, secretsCollection, "CreateItem", "a{sv}(oayays)b", request.Bytes())
	if err != nil {
		return secretCallError("store", err)
	}
	args := reply.Args()
	_ = args.String() // the item
	prompt := args.String()
	if args.Err() != nil {
		return args.Err()
	}
	return store.waitPrompt(prompt)
}

// LoadSecret reads the secret saved for the service and account.
func LoadSecret(service, account string) ([]byte, error) {
	store, err := openSecretStore()
	if err != nil {
		return nil, err
	}
	defer store.close()
	items, err := store.find(service, account)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, platform.ErrSecretNotFound
	}
	var request dbus.Encoder
	request.String(store.session)
	reply, err := store.conn.Call(secretsName, items[0], secretsItem, "GetSecret", "o", request.Bytes())
	if err != nil {
		return nil, secretCallError("read", err)
	}
	args := reply.Args()
	args.Align(8)
	_ = args.String() // the session
	end := args.ArrayLength(1)
	for args.Pos() < end && args.Err() == nil {
		args.Byte()
	}
	end = args.ArrayLength(1)
	var secret []byte
	for args.Pos() < end && args.Err() == nil {
		secret = append(secret, args.Byte())
	}
	_ = args.String() // the content type
	if args.Err() != nil {
		return nil, fmt.Errorf("linux: the Secret Service answered GetSecret badly: %w", args.Err())
	}
	return secret, nil
}

// DeleteSecret removes the secret saved for the service and account; one that
// is not there is not an error.
func DeleteSecret(service, account string) error {
	store, err := openSecretStore()
	if err != nil {
		return err
	}
	defer store.close()
	items, err := store.find(service, account)
	if err != nil {
		return err
	}
	for _, item := range items {
		reply, err := store.conn.Call(secretsName, item, secretsItem, "Delete", "", nil)
		if err != nil {
			return secretCallError("delete", err)
		}
		args := reply.Args()
		prompt := args.String()
		if args.Err() != nil {
			return args.Err()
		}
		if err := store.waitPrompt(prompt); err != nil && !strings.Contains(err.Error(), "dismissed") {
			return err
		}
	}
	return nil
}
