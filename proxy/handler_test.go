package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	dummyProxy   *Proxy
	dummyRequest *http.Request
	dummyURL     *url.URL
)

func init() {
	dummyProxy = &Proxy{}
	var err error
	dummyRequest, err = createDummyRequest()
	if err != nil {
		log.Fatal(err)
	}
}

func createDummyRequest() (*http.Request, error) {
	dummyHeaders := http.Header{
		"User-Agent":          {"dummy-client"},
		"X-Chocon-Test-Value": {"6"},
		// ignored headers
		"Connection":          {"Keep-Alive"},
		"Keep-Alive":          {"timeout=30, max=100"},
		"Proxy-Authenticate":  {"Basic"},
		"Proxy-Authorization": {"Basic dummy"},
		"Te":                  {"deflate"},
		"Trailers":            {"Expires"},
		"Transfer-Encoding":   {"chunked"},
		"Upgrade":             {"WebSocket"},
	}
	dummyURL = &url.URL{
		Scheme: "http",
		Path:   "/dummy",
	}
	req, err := http.NewRequest("GET", "/dummy", nil)
	if err != nil {
		return nil, err
	}
	req.Header = dummyHeaders
	req.Close = true
	req.Proto = "HTTP/1.0"
	req.ProtoMajor = 1
	req.ProtoMinor = 0
	req.URL = dummyURL

	return req, nil
}

func TestCopyRequest(t *testing.T) {
	req := dummyProxy.copyRequest(dummyRequest)

	assert.Equal(t, req.Proto, "HTTP/1.1")
	assert.Equal(t, req.ProtoMajor, 1)
	assert.Equal(t, req.ProtoMinor, 1)
	assert.Equal(t, req.Close, false)
	assert.Equal(t, req.URL.Scheme, "http")
	assert.Equal(t, req.URL.Path, dummyURL.Path)

	assert.Equal(t, req.Header["User-Agent"][0], "dummy-client")
	assert.Equal(t, req.Header["X-Chocon-Test-Value"][0], "6")

	for k, _ := range req.Header {
		if _, ok := ignoredHeaderNames[k]; ok {
			assert.Fail(t, fmt.Sprintf("header filed: %s must be removed", k))
		}
	}
}

func BenchmarkCopyRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = dummyProxy.copyRequest(dummyRequest)
	}
}
