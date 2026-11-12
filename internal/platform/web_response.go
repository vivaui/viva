package platform

import (
	"sort"
	"strconv"
	"strings"
)

// A custom scheme's answer has the same shape on every engine: a status, a type, headers merged over
// the ones the platform must send anyway, and a body. Resolving it here keeps the three backends from
// disagreeing about a 404, about a removed default, or about the order the headers are written in.

// WebAnswer is a WebResponse resolved against the platform's defaults: the fields each backend turns
// into the engine's own response object.
type WebAnswer struct {
	// Status is the HTTP status and Reason its reason phrase, empty for a code without a known one.
	Status int
	Reason string
	// MIME is the effective Content-Type, or empty when the application removed it.
	MIME string
	// Headers are the merged headers, their names canonicalised and sorted by name.
	Headers []WebHeader
	// Body is the body, nil for a response that carries none.
	Body []byte
}

// WebHeader is one header of an answer.
type WebHeader struct {
	Name, Value string
}

// webReasons are the reason phrases written into the status line. A code without one gets an empty
// phrase, which every engine accepts; the set covers what a scheme is likely to answer with rather
// than copying net/http's table into another linked package.
var webReasons = map[int]string{
	200: "OK",
	201: "Created",
	202: "Accepted",
	204: "No Content",
	206: "Partial Content",
	301: "Moved Permanently",
	302: "Found",
	304: "Not Modified",
	307: "Temporary Redirect",
	308: "Permanent Redirect",
	400: "Bad Request",
	401: "Unauthorized",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	416: "Range Not Satisfiable",
	500: "Internal Server Error",
	503: "Service Unavailable",
}

// ServeWebScheme answers path from the synchronous forms of scheme: Respond when it is set, otherwise
// Serve. It reports false when neither finds anything; AnswerWebResponse makes that a 404 unless the
// response names another status. ServeAsync is not consulted here, because the backend that calls it
// owns the engine's task until the answer arrives.
func ServeWebScheme(scheme *WebScheme, path string) (WebResponse, bool) {
	if scheme == nil {
		return WebResponse{}, false
	}
	if scheme.Respond != nil {
		return scheme.Respond(WebRequest{Path: path})
	}
	if scheme.Serve != nil {
		mime, data, ok := scheme.Serve(path)
		return WebResponse{MIME: mime, Data: data}, ok
	}
	return WebResponse{}, false
}

// AnswerWebResponse resolves response, which found or did not find the requested resource, into what
// every backend sends. A Status the application set wins, even when found is false; otherwise found is
// 200 and not found is a 404. A 404 carries whatever body and MIME the answer names, like every other
// status: an empty MIME is text/plain for a 404 and application/octet-stream otherwise.
func AnswerWebResponse(response WebResponse, found bool) WebAnswer {
	status := response.Status
	switch {
	case status == 0 && found:
		status = 200
	case status == 0:
		status = 404
	case status < 100 || status > 999:
		// The engines refuse a status outside the HTTP range; an application that computed one
		// should see a failure rather than a success.
		status = 500
	}
	mime := response.MIME
	if mime == "" {
		if status == 404 {
			mime = "text/plain"
		} else {
			mime = "application/octet-stream"
		}
	}
	body := response.Data
	if status == 204 || status == 304 {
		body = nil
	}
	// The defaults are the headers the platform used to add on its own, so an application that sets
	// nothing keeps the responses it had.
	merged := map[string]string{
		"Content-Type":                mime,
		"Content-Length":              strconv.Itoa(len(body)),
		"Access-Control-Allow-Origin": "*",
	}
	for name, value := range response.Headers {
		name = headerName(name)
		if name == "" {
			continue
		}
		if value == "" {
			delete(merged, name)
			continue
		}
		merged[name] = value
	}
	headers := make([]WebHeader, 0, len(merged))
	for name, value := range merged {
		headers = append(headers, WebHeader{Name: name, Value: value})
	}
	sort.Slice(headers, func(i, j int) bool { return headers[i].Name < headers[j].Name })
	return WebAnswer{Status: status, Reason: webReasons[status], MIME: merged["Content-Type"], Headers: headers, Body: body}
}

// headerName is the canonical form of a header name, or empty when the name is not a token: a name
// with a space, a colon or a line break cannot survive the header block the engines are given.
//
// The canonical form is net/textproto's — an upper-case letter first and after every "-", lower case
// elsewhere — written here rather than imported: net/textproto links the net package into every
// application that links app, a hundred kilobytes of resolver and socket code that a scheme header
// never uses.
func headerName(name string) string {
	if name == "" {
		return ""
	}
	canonical := []byte(name)
	upper := true
	for index, character := range canonical {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		if !letter && (character < '0' || character > '9') && !headerTokenByte(character) {
			return ""
		}
		switch {
		case upper && character >= 'a' && character <= 'z':
			canonical[index] = character - 'a' + 'A'
		case !upper && character >= 'A' && character <= 'Z':
			canonical[index] = character - 'A' + 'a'
		}
		upper = character == '-'
	}
	return string(canonical)
}

// headerTokenByte reports whether character may appear in an HTTP field name (RFC 9110's tchar)
// besides the letters and digits headerName has already accepted.
func headerTokenByte(character byte) bool {
	switch character {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

// HeaderBlock is the headers as the lines WebView2 takes and the Darwin bridge parses, in the sorted
// order, so an answer is the same on every run.
func (answer WebAnswer) HeaderBlock() string {
	lines := make([]string, len(answer.Headers))
	for index, header := range answer.Headers {
		lines[index] = header.Name + ": " + header.Value
	}
	return strings.Join(lines, "\r\n")
}
