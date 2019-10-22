package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/kazeburo/chocon/upstream"
	"github.com/rs/xid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

const (
	httpStatusClientClosedRequest = 499
)

var (
	proxyVerHeader = []byte("X-Chocon-Ver")
	proxyIDHeader  = []byte("X-Chocon-Id")
)

// These headers won't be copied from original request to proxy request.
var ignoredHeaderNames = [][]byte{
	[]byte("Connection"),
	[]byte("Keep-Alive"),
	[]byte("Proxy-Authenticate"),
	[]byte("Proxy-Authorization"),
	[]byte("Te"),
	[]byte("Trailers"),
	[]byte("Transfer-Encoding"),
	[]byte("Upgrade"),
}

// Status for override http status
type Status struct {
	Code int
}

// Proxy : Provide host-based proxy server.
type Proxy struct {
	Version  string
	Client   *fasthttp.Client
	logger   *zap.Logger
	upstream *upstream.Upstream
}

var errInvalidHostHeader = errors.New("invalid host value in header")

// New sets up a proxy.
func New(client *fasthttp.Client, version string, logger *zap.Logger, upstream *upstream.Upstream) *Proxy {
	return &Proxy{
		Version:  version,
		Client:   client,
		logger:   logger,
		upstream: upstream,
	}
}

// Rewrite the request's host header based on the value of the original request's host header.
// If the original request's host header is 'example.com.ccnproxy.local',
// the request's host header is set to 'example.com'
//
// May return errInvalidHostHeader.
func rewriteHost(req *fasthttp.Request, originalReq *fasthttp.Request) error {
	if len(originalReq.Host()) == 0 {
		return errInvalidHostHeader
	}
	hostPortSplit := bytes.Split(originalReq.Host(), []byte(":"))
	host := hostPortSplit[0]
	port := []byte("")
	if len(hostPortSplit) > 1 {
		port = append([]byte(":"), hostPortSplit[1]...)
	}
	hostSplit := bytes.Split(host, []byte("."))
	lastPartIndex := 0
	for i, hostPart := range hostSplit {
		if bytes.Equal(hostPart, []byte("ccnproxy-ssl")) || bytes.Equal(hostPart, []byte("ccnproxy-secure")) || bytes.Equal(hostPart, []byte("ccnproxy-https")) || bytes.Equal(hostPart, []byte("ccnproxy")) {
			lastPartIndex = i
		}
	}

	if lastPartIndex == 0 {
		return errInvalidHostHeader
	}

	req.URI().SetHostBytes(append(bytes.Join(hostSplit[0:lastPartIndex], []byte(".")), port...))

	if bytes.Equal(hostSplit[lastPartIndex], []byte("ccnproxy-https")) || bytes.Equal(hostSplit[lastPartIndex], []byte("ccnproxy-secure")) || bytes.Equal(hostSplit[lastPartIndex], []byte("ccnproxy-ssl")) {
		req.URI().SetSchemeBytes([]byte("https"))
	}

	return nil
}

// Handler proxies requests. It reads a request from the client,
// makes a request to the target server, and sends its response
// to the client.
func (p *Proxy) Handler(ctx *fasthttp.RequestCtx) {
	if len(ctx.Request.Header.PeekBytes(proxyVerHeader)) > 0 {
		ctx.SetStatusCode(http.StatusLoopDetected)
		return
	}

	proxyID := ctx.Request.Header.PeekBytes(proxyIDHeader)

	if len(proxyID) == 0 {
		// `xid.New().Bytes()` does not work as we want
		// the []byte representation of a string.
		proxyID = []byte(xid.New().String())
	}

	ctx.Response.Header.SetBytesKV(proxyIDHeader, proxyID)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	// Copy the original request with some tweaks.
	ctx.Request.CopyTo(req)
	for _, n := range ignoredHeaderNames {
		req.Header.DelBytes(n)
	}

	if p.upstream.Enabled() {
		h, ipwc, err := p.upstream.Get()
		defer p.upstream.Release(ipwc)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
			return
		}
		uri := req.URI()
		uri.SetScheme(p.upstream.GetScheme())
		uri.SetHost(h)
	} else {
		ctx.Request.Header.SetBytesK(proxyVerHeader, p.Version)

		// Rewrite the request's host header.
		if err := rewriteHost(req, &ctx.Request); err != nil {
			// If the original request's host header is invalid, return 400.
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			return
		}
	}

	// Make a request to the target server.
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	if err := p.Client.Do(req, res); err != nil {
		logger := p.logger.With(
			zap.ByteString("request_host", ctx.Host()),
			zap.ByteString("request_path", ctx.Path()),
			zap.ByteString("proxy_host", req.Host()),
			zap.ByteString("proxy_scheme", req.URI().Scheme()),
			zap.ByteString("proxy_id", proxyID),
		)

		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			logger.Error("ErrorFromProxy", zap.Error(err))
			ctx.SetStatusCode(fasthttp.StatusGatewayTimeout)
		} else if err == context.Canceled || err == io.ErrUnexpectedEOF {
			logger.Error("ErrorFromProxy",
				zap.Error(fmt.Errorf("%v: seems client closed request", err)))
			ctx.SetContentType("text/plain; charset=utf-8")
			ctx.Response.Header.Set("x-content-type-options", "nosniff")
			ctx.SetStatusCode(httpStatusClientClosedRequest)
			ctx.WriteString("client closed request")
		} else {
			logger.Error("ErrorFromProxy", zap.Error(err))
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
		}

		return
	}

	// Copy the target server's response and sends it back to the client.
	ctx.SetBody(res.Body())
	ctx.SetStatusCode(res.StatusCode())
	res.Header.VisitAll(func(k, v []byte) {
		if !bytes.Equal([]byte(proxyIDHeader), k) {
			ctx.Response.Header.SetBytesKV(k, v)
		}
	})
}
