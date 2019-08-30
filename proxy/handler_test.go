package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/valyala/fasthttp"
)

func TestRewriteHost(t *testing.T) {
	cases := []struct {
		originalReqHost string
		reqHost         string
		scheme          string
	}{
		{"example.com.ccnproxy:3000", "example.com:3000", "http"},
		{"example.com.ccnproxy", "example.com", "http"},
		{"example.com.ccnproxy.local:3000", "example.com:3000", "http"},
		{"example.com.ccnproxy.local", "example.com", "http"},
		{"example.com.ccnproxy-ssl:3000", "example.com:3000", "https"},
		{"example.com.ccnproxy-ssl", "example.com", "https"},
	}

	for _, c := range cases {
		t.Run(c.originalReqHost, func(t *testing.T) {
			originalReq := fasthttp.AcquireRequest()
			req := fasthttp.AcquireRequest()
			originalReq.SetHost(c.originalReqHost)
			rewriteHost(req, originalReq)
			assert.Equal(t, string(req.Host()), c.reqHost)
			assert.Equal(t, string(req.URI().Scheme()), c.scheme)
		})
	}
}
