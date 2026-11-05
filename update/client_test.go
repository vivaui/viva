package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type signedFixture struct {
	body      []byte
	signature string
	public    ed25519.PublicKey
}

func signFeed(t *testing.T, feed Feed) signedFixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := Sign(body, private)
	if err != nil {
		t.Fatal(err)
	}
	return signedFixture{body: body, signature: signature, public: public}
}

func serveFixture(t *testing.T, fixture signedFixture) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(fixture.body)
	})
	mux.HandleFunc("/latest.json.sig", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, fixture.signature)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	return server
}

func fixtureClient(t *testing.T, server *httptest.Server, fixture signedFixture, current string) *Client {
	t.Helper()
	client, err := New(Config{
		FeedURL:    server.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{fixture.public},
		Current:    current,
		HTTP:       server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestCheckFindsANewerRelease(t *testing.T) {
	fixture := signFeed(t, validFeed())
	server := serveFixture(t, fixture)
	client := fixtureClient(t, server, fixture, "1.0.0")

	feed, err := client.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if feed == nil {
		t.Fatal("Check found no update")
	}
	if feed.Latest.Version != "1.4.2" {
		t.Errorf("version = %q, want 1.4.2", feed.Latest.Version)
	}
	asset, err := feed.Asset("darwin", "arm64")
	if err != nil {
		t.Fatalf("Asset: %v", err)
	}
	if asset.Format != FormatZIP {
		t.Errorf("asset format = %q, want %q", asset.Format, FormatZIP)
	}
}

func TestCheckFindsNothingWhenCurrentIsNotOlder(t *testing.T) {
	fixture := signFeed(t, validFeed())
	server := serveFixture(t, fixture)
	for _, current := range []string{"1.4.2", "1.10.0", "2.0.0"} {
		client := fixtureClient(t, server, fixture, current)
		feed, err := client.Check(context.Background())
		if err != nil {
			t.Fatalf("Check at %s: %v", current, err)
		}
		if feed != nil {
			t.Errorf("Check at %s offered %s", current, feed.Latest.Version)
		}
	}
}

func TestCheckRefusesAnotherApplicationOrChannel(t *testing.T) {
	fixture := signFeed(t, validFeed())
	server := serveFixture(t, fixture)

	for name, config := range map[string]Config{
		"app": {
			FeedURL:    server.URL + "/latest.json",
			PublicKeys: []ed25519.PublicKey{fixture.public},
			App:        "dev.example.other",
			Current:    "1.0.0",
			HTTP:       server.Client(),
		},
		"channel": {
			FeedURL:    server.URL + "/latest.json",
			PublicKeys: []ed25519.PublicKey{fixture.public},
			Channel:    "beta",
			Current:    "1.0.0",
			HTTP:       server.Client(),
		},
	} {
		client, err := New(config)
		if err != nil {
			t.Fatalf("%s: New: %v", name, err)
		}
		if _, err := client.Check(context.Background()); !errors.Is(err, ErrManifest) {
			t.Errorf("%s: Check error %v does not wrap ErrManifest", name, err)
		}
	}
}

func TestCheckRefusesAnotherSignature(t *testing.T) {
	fixture := signFeed(t, validFeed())
	server := serveFixture(t, fixture)

	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := New(Config{
		FeedURL:    server.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{other},
		Current:    "1.0.0",
		HTTP:       server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrongKey.Check(context.Background()); !errors.Is(err, ErrSignature) {
		t.Errorf("wrong key: error %v does not wrap ErrSignature", err)
	}

	tampered := fixture
	tampered.body = append(append([]byte{}, fixture.body...), '\n')
	server = serveFixture(t, tampered)
	client := fixtureClient(t, server, fixture, "1.0.0")
	if _, err := client.Check(context.Background()); !errors.Is(err, ErrSignature) {
		t.Errorf("tampered body: error %v does not wrap ErrSignature", err)
	}

	malformed := fixture
	malformed.signature = "not base64!"
	server = serveFixture(t, malformed)
	client = fixtureClient(t, server, malformed, "1.0.0")
	if _, err := client.Check(context.Background()); !errors.Is(err, ErrSignature) {
		t.Errorf("malformed signature: error %v does not wrap ErrSignature", err)
	}
}

func TestCheckNeedsBothFiles(t *testing.T) {
	fixture := signFeed(t, validFeed())
	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(fixture.body)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	client := fixtureClient(t, server, fixture, "1.0.0")
	if _, err := client.Check(context.Background()); err == nil {
		t.Fatal("Check accepted a feed without a signature")
	}
}

func TestCheckRefusesAnOversizedFeed(t *testing.T) {
	body := bytes.Repeat([]byte("x"), int(DefaultMaxManifestBytes)+1)
	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(body)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	client, err := New(Config{
		FeedURL:    server.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{make(ed25519.PublicKey, ed25519.PublicKeySize)},
		Current:    "1.0.0",
		HTTP:       server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Check(context.Background()); !errors.Is(err, ErrTooLarge) {
		t.Errorf("error %v does not wrap ErrTooLarge", err)
	}
}

func TestCheckRefusesARedirectThatLeavesHTTPS(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, "not signed")
	}))
	t.Cleanup(plain.Close)

	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, plain.URL, http.StatusFound)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	client, err := New(Config{
		FeedURL:    server.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{make(ed25519.PublicKey, ed25519.PublicKeySize)},
		Current:    "1.0.0",
		HTTP:       server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Check(context.Background()); !errors.Is(err, ErrInsecureURL) {
		t.Errorf("error %v does not wrap ErrInsecureURL", err)
	}
}

func TestNewRefusesWhatCannotWork(t *testing.T) {
	key := make(ed25519.PublicKey, ed25519.PublicKeySize)
	tests := map[string]Config{
		"no feed":     {PublicKeys: []ed25519.PublicKey{key}, Current: "1.0.0"},
		"http feed":   {FeedURL: "http://example.com/latest.json", PublicKeys: []ed25519.PublicKey{key}, Current: "1.0.0"},
		"no keys":     {FeedURL: "https://example.com/latest.json", Current: "1.0.0"},
		"short key":   {FeedURL: "https://example.com/latest.json", PublicKeys: []ed25519.PublicKey{{1, 2}}, Current: "1.0.0"},
		"bad current": {FeedURL: "https://example.com/latest.json", PublicKeys: []ed25519.PublicKey{key}, Current: "newest"},
	}
	for name, config := range tests {
		if _, err := New(config); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
}

func TestCheckReportsAnErrorStatus(t *testing.T) {
	fixture := signFeed(t, validFeed())
	mux := http.NewServeMux()
	mux.HandleFunc("/latest.json", func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "gone", http.StatusGone)
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	client := fixtureClient(t, server, fixture, "1.0.0")
	_, err := client.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "410") {
		t.Errorf("error = %v, want a 410 status", err)
	}
}

func TestDecodeSignatureRefusesTheWrongShape(t *testing.T) {
	for _, text := range []string{"", "  ", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := decodeSignature([]byte(text)); !errors.Is(err, ErrSignature) {
			t.Errorf("decodeSignature(%q) error %v does not wrap ErrSignature", text, err)
		}
	}
}

func TestSignatureURLKeepsTheQuery(t *testing.T) {
	got, err := signatureURLFor("https://example.com/apps/latest.json?channel=beta")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.com/apps/latest.json.sig?channel=beta"; got != want {
		t.Errorf("signatureURLFor = %q, want %q", got, want)
	}
}

func TestCheckRefusesAnExpiredFeed(t *testing.T) {
	feed := validFeed()
	feed.Latest.Version = "2.0.0"
	fixture := signFeed(t, feed)
	server := serveFixture(t, fixture)
	client, err := New(Config{
		FeedURL:    server.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{fixture.public},
		Current:    "1.0.0",
		HTTP:       server.Client(),
		Now:        func() time.Time { return feed.Expires.Add(time.Second) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Check(context.Background()); !errors.Is(err, ErrExpired) || !errors.Is(err, ErrManifest) {
		t.Errorf("error %v does not wrap ErrExpired and ErrManifest", err)
	}
}

// A redirect to plain HTTP is refused before the request is sent, so nothing
// reaches the insecure host.
func TestRedirectToHTTPIsNeverRequested(t *testing.T) {
	var reached bool
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	t.Cleanup(plain.Close)
	secure := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, plain.URL, http.StatusFound)
	}))
	t.Cleanup(secure.Close)
	client, err := New(Config{
		FeedURL:    secure.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{make(ed25519.PublicKey, ed25519.PublicKeySize)},
		Current:    "1.0.0",
		HTTP:       secure.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Check(context.Background()); !errors.Is(err, ErrInsecureURL) {
		t.Errorf("error %v does not wrap ErrInsecureURL", err)
	}
	if reached {
		t.Error("the plain HTTP server received a request")
	}
}
