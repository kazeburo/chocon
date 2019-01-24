package proxy

// this class is based on https://github.com/r7kamura/entoverse

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/renstrom/shortuuid"
	"go.uber.org/zap"
)

const (
	proxyHeaderName = "X-Chocon-Req"
)

// These headers won't be copied from original request to proxy request.
var ignoredHeaderNames = map[string]struct{}{
	"Connection":          struct{}{},
	"Keep-Alive":          struct{}{},
	"Proxy-Authenticate":  struct{}{},
	"Proxy-Authorization": struct{}{},
	"Te":                  struct{}{},
	"Trailers":            struct{}{},
	"Transfer-Encoding":   struct{}{},
	"Upgrade":             struct{}{},
}

// Status for override http status
type Status struct {
	Code int
}

// Proxy : Provide host-based proxy server.
type Proxy struct {
	Transport   http.RoundTripper
	upstreamURL url.URL
	logger      *zap.Logger
}

// New :  Create a request-based reverse-proxy.
func New(transport *http.RoundTripper, upstreamURL *url.URL, logger *zap.Logger) *Proxy {
	return &Proxy{
		Transport:   *transport,
		upstreamURL: *upstreamURL,
		logger:      logger,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, originalRequest *http.Request) {
	// If request has Via: ViaHeader, stop request
	if originalRequest.Header.Get(proxyHeaderName) != "" {
		writer.WriteHeader(http.StatusLoopDetected)
		return
	}

	proxyID := shortuuid.New()

	// Create a new proxy request object by coping the original request.
	proxyRequest := proxy.copyRequest(originalRequest)
	status := &Status{Code: http.StatusOK}

	if proxy.upstreamURL.Scheme == "" {
		// Set Proxied
		originalRequest.Header.Set(proxyHeaderName, proxyID)
		// Convert an original request into another proxy request.
		proxy.rewriteProxyHost(originalRequest, proxyRequest, status)
	} else {
		proxyRequest.URL.Scheme = proxy.upstreamURL.Scheme
		proxyRequest.URL.Host = proxy.upstreamURL.Host
		proxyRequest.Host = originalRequest.Host
	}
	if status.Code != http.StatusOK {
		writer.WriteHeader(status.Code)
		return
	}

	// Convert a request into a response by using its Transport.
	response, err := proxy.Transport.RoundTrip(proxyRequest)
	if err != nil {
		proxy.logger.Error("ErrorFromProxy", zap.Error(err))
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
	writer.Header().Add(proxyHeaderName, proxyID)

	// Copy a status code.
	writer.WriteHeader(response.StatusCode)

	// Copy a response body.
	io.Copy(writer, response.Body)
}

func (proxy *Proxy) rewriteProxyHost(r *http.Request, pr *http.Request, ps *Status) {
	if r.Host == "" {
		ps.Code = http.StatusBadRequest
		return
	}
	hostPortSplit := strings.Split(r.Host, ":")
	host := hostPortSplit[0]
	port := ""
	if len(hostPortSplit) > 1 {
		port = ":" + hostPortSplit[1]
	}
	hostSplit := strings.Split(host, ".")
	lastPartIndex := 0
	for i, hostPart := range hostSplit {
		if hostPart == "ccnproxy-ssl" || hostPart == "ccnproxy-secure" || hostPart == "ccnproxy-https" || hostPart == "ccnproxy" {
			lastPartIndex = i
		}
	}
	if lastPartIndex == 0 {
		ps.Code = http.StatusBadRequest
		return
	}

	pr.URL.Host = strings.Join(hostSplit[0:lastPartIndex], ".") + port
	pr.Host = pr.URL.Host
	if hostSplit[lastPartIndex] == "ccnproxy-https" || hostSplit[lastPartIndex] == "ccnproxy-secure" || hostSplit[lastPartIndex] == "ccnproxy-ssl" {
		pr.URL.Scheme = "https"
	}
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

	// Copy all header fields except ignoredHeaderNames'.
	for key, values := range originalRequest.Header {
		if _, ok := ignoredHeaderNames[key]; ok {
			continue
		}
		for _, value := range values {
			proxyRequest.Header.Add(key, value)
		}
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
