package proxy

// this class is based on https://github.com/r7kamura/entoverse

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/renstrom/shortuuid"
)

const (
	ProxyHeaderName = "X-Chocon-Req"
)

// These headers won't be copied from original request to proxy request.
var ignoredHeaderNames = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

type ProxyStatus struct {
	Status int
}

// Provide host-based proxy server.
type Proxy struct {
	RequestConverter func(originalRequest, proxyRequest *http.Request, proxyStatus *ProxyStatus)
	Transport        http.RoundTripper
}

// Create a request-based reverse-proxy.
func NewProxyWithRequestConverter(requestConverter func(*http.Request, *http.Request, *ProxyStatus), transport *http.RoundTripper) *Proxy {
	return &Proxy{
		RequestConverter: requestConverter,
		Transport:        *transport,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, originalRequest *http.Request) {
	// If request has Via: ViaHeader, stop request
	if originalRequest.Header.Get(ProxyHeaderName) != "" {
		writer.WriteHeader(http.StatusLoopDetected)
		return
	}

	// Set Proxied
	proxyID := shortuuid.New()
	originalRequest.Header.Set(ProxyHeaderName, proxyID)

	// Create a new proxy request object by coping the original request.
	proxyRequest := proxy.copyRequest(originalRequest)
	proxyStatus := &ProxyStatus{Status: http.StatusOK}

	// Convert an original request into another proxy request.
	proxy.RequestConverter(originalRequest, proxyRequest, proxyStatus)

	if proxyStatus.Status != http.StatusOK {
		writer.WriteHeader(proxyStatus.Status)
		return
	}

	// Convert a request into a response by using its Transport.
	response, err := proxy.Transport.RoundTrip(proxyRequest)
	if err != nil {
		log.Printf("ErrorFromProxy: %v", err)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			writer.WriteHeader(http.StatusGatewayTimeout)
		} else {
			writer.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Ensure a response body from upstream will be always closed.
	defer response.Body.Close()

	// Copy all header fields.
	for key, values := range response.Header {
		for _, value := range values {
			writer.Header().Add(key, value)
		}
	}
	writer.Header().Add(ProxyHeaderName, proxyID)

	// Copy a status code.
	writer.WriteHeader(response.StatusCode)

	// Copy a response body.
	io.Copy(writer, response.Body)
}

// Create a new proxy request with some modifications from an original request.
func (proxy *Proxy) copyRequest(originalRequest *http.Request) *http.Request {
	proxyRequest := new(http.Request)
	proxyURL := new(url.URL)
	*proxyRequest = *originalRequest
	*proxyURL = *originalRequest.URL
	proxyRequest.URL = proxyURL
	proxyRequest.Proto = "HTTP/1.1"
	proxyRequest.ProtoMajor = 1
	proxyRequest.ProtoMinor = 1
	proxyRequest.Close = false
	proxyRequest.Header = make(http.Header)
	proxyRequest.URL.Scheme = "http"
	proxyRequest.URL.Path = originalRequest.URL.Path

	// Copy all header fields.
	for key, values := range originalRequest.Header {
		for _, value := range values {
			proxyRequest.Header.Add(key, value)
		}
	}

	// Remove ignored header fields.
	for _, headerName := range ignoredHeaderNames {
		proxyRequest.Header.Del(headerName)
	}

	// Append this machine's host name into X-Forwarded-For.
	// if requestHost, _, err := net.SplitHostPort(originalRequest.RemoteAddr); err == nil {
	// 	if originalValues, ok := proxyRequest.Header["X-Forwarded-For"]; ok {
	// 		requestHost = strings.Join(originalValues, ", ") + ", " + requestHost
	// 	}
	// 	proxyRequest.Header.Set("X-Forwarded-For", requestHost)
	// }

	return proxyRequest
}
