package proxy

// this class is based on https://github.com/r7kamura/entoverse

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/kazeburo/chocon/upstream"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

const (
	proxyVerHeader                = "X-Chocon-Ver"
	proxyIDHeader                 = "X-Chocon-Id"
	httpStatusClientClosedRequest = 499
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
	Version   string
	Transport http.RoundTripper
	upstream  *upstream.Upstream
	logger    *zap.Logger
}

var pool = sync.Pool{
	New: func() interface{} { return make([]byte, 32*1024) },
}

// New :  Create a request-based reverse-proxy.
func New(transport *http.RoundTripper, version string, upstream *upstream.Upstream, logger *zap.Logger) *Proxy {
	return &Proxy{
		Version:   version,
		Transport: *transport,
		upstream:  upstream,
		logger:    logger,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, originalRequest *http.Request) {
	proxyID := originalRequest.Header.Get(proxyIDHeader)
	if proxyID == "" {
		proxyID = xid.New().String()
		originalRequest.Header.Set(proxyIDHeader, proxyID)
	}
	writer.Header().Set(proxyIDHeader, proxyID)

	// If request has Via: ViaHeader, stop request
	if originalRequest.Header.Get(proxyVerHeader) != "" {
		writer.WriteHeader(http.StatusLoopDetected)
		return
	}

	// Create a new proxy request object by coping the original request.
	proxyRequest := proxy.copyRequest(originalRequest)
	status := &Status{Code: http.StatusOK}

	if proxy.upstream.Enabled() {
		h, ipwc, err := proxy.upstream.Get()
		defer proxy.upstream.Release(ipwc)
		if err != nil {
			status.Code = http.StatusBadGateway
		}
		proxyRequest.URL.Scheme = proxy.upstream.GetScheme()
		proxyRequest.URL.Host = h
		proxyRequest.Host = originalRequest.Host
	} else {
		// Set Proxied
		originalRequest.Header.Set(proxyVerHeader, proxy.Version)
		// Convert an original request into another proxy request.
		proxy.rewriteProxyHost(originalRequest, proxyRequest, status)
	}
	if status.Code != http.StatusOK {
		writer.WriteHeader(status.Code)
		return
	}

	logger := proxy.logger.With(
		zap.String("request_host", originalRequest.Host),
		zap.String("request_path", originalRequest.URL.Path),
		zap.String("proxy_host", proxyRequest.URL.Host),
		zap.String("proxy_scheme", proxyRequest.URL.Scheme),
		zap.String("proxy_id", proxyID),
	)

	// Convert a request into a response by using its Transport.
	response, err := proxy.Transport.RoundTrip(proxyRequest)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			logger.Error("ErrorFromProxy", zap.Error(err))
			writer.WriteHeader(http.StatusGatewayTimeout)
		} else if err == context.Canceled || err == io.ErrUnexpectedEOF {
			logger.Error("ErrorFromProxy",
				zap.Error(fmt.Errorf("%v: seems client closed request", err)),
			)
			// For custom status code
			http.Error(writer, "Client Closed Request", httpStatusClientClosedRequest)
		} else {
			logger.Error("ErrorFromProxy", zap.Error(err))
			writer.WriteHeader(http.StatusBadGateway)
		}
		return
	}

	buf := pool.Get().([]byte)
	defer func() {
		response.Body.Close()
		pool.Put(buf)
	}()

	// Copy all header fields.
	nv := 0
	for _, vv := range response.Header {
		nv += len(vv)
	}
	sv := make([]string, nv)
	for k, vv := range response.Header {
		if k == proxyIDHeader {
			continue
		}
		n := copy(sv, vv)
		writer.Header()[k] = sv[:n:n]
		sv = sv[n:]
	}

	// Copy a status code.
	writer.WriteHeader(response.StatusCode)

	// Copy a response body.
	io.CopyBuffer(writer, response.Body, buf)
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
	nv := 0
	for _, vv := range originalRequest.Header {
		nv += len(vv)
	}
	sv := make([]string, nv)
	for k, vv := range originalRequest.Header {
		if _, ok := ignoredHeaderNames[k]; ok {
			continue
		}
		n := copy(sv, vv)
		proxyRequest.Header[k] = sv[:n:n]
		sv = sv[n:]
	}

	return proxyRequest
}
