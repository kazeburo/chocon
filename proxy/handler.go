package proxy

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rs/xid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

const (
	proxyVerHeader                = "X-Chocon-Ver"
	proxyIDHeader                 = "X-Chocon-Id"
	httpStatusClientClosedRequest = 499
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
	Version string
	Client  *fasthttp.Client
	logger  *zap.Logger
}

var errInvalidHostHeader = errors.New("invalid host value in header")

// New sets up a proxy.
func New(client *fasthttp.Client, version string, logger *zap.Logger) *Proxy {
	return &Proxy{
		Version: version,
		Client:  client,
		logger:  logger,
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

	hostPortSplit := strings.Split(string(originalReq.Host()), ":")

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
		return errInvalidHostHeader
	}

	req.URI().SetHost(strings.Join(hostSplit[0:lastPartIndex], ".") + port)
	req.SetHostBytes(req.URI().Host())

	if hostSplit[lastPartIndex] == "ccnproxy-https" || hostSplit[lastPartIndex] == "ccnproxy-secure" || hostSplit[lastPartIndex] == "ccnproxy-ssl" {
		req.URI().SetScheme("https")
	}

	return nil
}

// Handler proxies requests. It reads a request from the client,
// makes a request to the target server, and sends its response
// to the client.
func (p *Proxy) Handler(ctx *fasthttp.RequestCtx) {
	if len(ctx.Request.Header.Peek(proxyVerHeader)) > 0 {
		ctx.SetStatusCode(http.StatusLoopDetected)
		return
	}

	proxyID := ctx.Request.Header.Peek(proxyIDHeader)

	if len(proxyID) == 0 {
		// `xid.New().Bytes()` does not work as we want
		// the []byte representation of a string.
		proxyID = []byte(xid.New().String())
	}

	ctx.Response.Header.SetBytesV(proxyIDHeader, proxyID)

	// Copy the original request with some tweaks.
	req := fasthttp.AcquireRequest()
	ctx.Request.CopyTo(req)
	for _, n := range ignoredHeaderNames {
		req.Header.DelBytes(n)
	}
	ctx.Request.Header.Set(proxyVerHeader, p.Version)

	// Rewrite the request's host header.
	if err := rewriteHost(req, &ctx.Request); err != nil {
		// If the original request's host header is invalid, return 400.
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	// Make a request to the target server.

	res := fasthttp.AcquireResponse()

	if err := p.Client.Do(req, res); err != nil {
		logger := p.logger.With(
			zap.ByteString("request_host", ctx.Host()),
			zap.ByteString("request_path", ctx.Path()),
			zap.ByteString("proxy_host", req.Host()),
			zap.ByteString("proxy_scheme", req.URI().Scheme()),
			zap.ByteString("proxy_id", proxyID),
		)

		// TODO: change the message depending on the type of error.
		logger.Error("Error from proxy", zap.Error(err))
	}

	fasthttp.ReleaseRequest(req)

	// Copy the target server's response and sends it back to the client.
	ctx.SetBody(res.Body())
	ctx.SetStatusCode(res.StatusCode())

	fasthttp.ReleaseResponse(res)
}
