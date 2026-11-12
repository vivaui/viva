package platform

import (
	"slices"
	"strings"
	"testing"
)

func TestAnswerWebResponseResolvesEveryEngineTheSameWay(t *testing.T) {
	notFoundHeaders := []WebHeader{
		{Name: "Access-Control-Allow-Origin", Value: "*"},
		{Name: "Content-Length", Value: "0"},
		{Name: "Content-Type", Value: "text/plain"},
	}
	for _, test := range []struct {
		name     string
		response WebResponse
		found    bool
		status   int
		reason   string
		mime     string
		headers  []WebHeader
		body     string
		noBody   bool
	}{
		{
			name:     "the defaults of a found resource",
			response: WebResponse{MIME: "text/html; charset=utf-8", Data: []byte("hi")},
			found:    true, status: 200, reason: "OK", mime: "text/html; charset=utf-8", body: "hi",
			headers: []WebHeader{
				{Name: "Access-Control-Allow-Origin", Value: "*"},
				{Name: "Content-Length", Value: "2"},
				{Name: "Content-Type", Value: "text/html; charset=utf-8"},
			},
		},
		{
			name: "a resource that was not found", found: false,
			status: 404, reason: "Not Found", mime: "text/plain", noBody: true,
			headers: notFoundHeaders,
		},
		{
			name: "headers merged, canonicalised and removed",
			response: WebResponse{
				Status: 201,
				MIME:   "text/html",
				Headers: map[string]string{
					"content-type":                "application/xhtml+xml",
					"cache-control":               "no-store",
					"X-Content-Type-Options":      "nosniff",
					"Access-Control-Allow-Origin": "",        // removes the default
					"Content-Length":              "",        // removes the default
					"Two Words":                   "dropped", // not a header name
				},
				Data: []byte("hello"),
			},
			found: true, status: 201, reason: "Created", mime: "application/xhtml+xml", body: "hello",
			headers: []WebHeader{
				{Name: "Cache-Control", Value: "no-store"},
				{Name: "Content-Type", Value: "application/xhtml+xml"},
				{Name: "X-Content-Type-Options", Value: "nosniff"},
			},
		},
		{
			name:     "no content carries no body",
			response: WebResponse{Status: 204, Data: []byte("not sent")},
			found:    true, status: 204, reason: "No Content", mime: "application/octet-stream", noBody: true,
			headers: []WebHeader{
				{Name: "Access-Control-Allow-Origin", Value: "*"},
				{Name: "Content-Length", Value: "0"},
				{Name: "Content-Type", Value: "application/octet-stream"},
			},
		},
		{
			name:     "not modified carries no body",
			response: WebResponse{Status: 304, Data: []byte("not sent")},
			found:    true, status: 304, reason: "Not Modified", mime: "application/octet-stream", noBody: true,
			headers: []WebHeader{
				{Name: "Access-Control-Allow-Origin", Value: "*"},
				{Name: "Content-Length", Value: "0"},
				{Name: "Content-Type", Value: "application/octet-stream"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			answer := AnswerWebResponse(test.response, test.found)
			if answer.Status != test.status || answer.Reason != test.reason || answer.MIME != test.mime {
				t.Fatalf("answer = %+v, want status %d, reason %q, MIME %q", answer, test.status, test.reason, test.mime)
			}
			if !slices.Equal(answer.Headers, test.headers) {
				t.Fatalf("headers = %v, want %v", answer.Headers, test.headers)
			}
			if test.noBody {
				if answer.Body != nil {
					t.Fatalf("body = %q, want none", answer.Body)
				}
			} else if string(answer.Body) != test.body {
				t.Fatalf("body = %q, want %q", answer.Body, test.body)
			}
			lines := make([]string, len(test.headers))
			for index, header := range test.headers {
				lines[index] = header.Name + ": " + header.Value
			}
			if block := answer.HeaderBlock(); block != strings.Join(lines, "\r\n") {
				t.Fatalf("header block = %q, want %q", block, strings.Join(lines, "\r\n"))
			}
		})
	}
}

func TestServeWebSchemePrefersRespondThenServe(t *testing.T) {
	scheme := &WebScheme{
		Serve: func(path string) (string, []byte, bool) {
			return "text/plain", []byte("serve " + path), true
		},
		Respond: func(request WebRequest) (WebResponse, bool) {
			return WebResponse{MIME: "text/html", Data: []byte("respond " + request.Path)}, true
		},
	}
	response, found := ServeWebScheme(scheme, "index.html")
	if !found || string(response.Data) != "respond index.html" {
		t.Fatalf("Respond was not preferred: %q, %v", response.Data, found)
	}
	scheme.Respond = nil
	response, found = ServeWebScheme(scheme, "index.html")
	if !found || string(response.Data) != "serve index.html" {
		t.Fatalf("Serve was not used: %q, %v", response.Data, found)
	}
	if _, found := ServeWebScheme(nil, "index.html"); found {
		t.Fatal("a nil scheme found a resource")
	}
}
