package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kazeburo/chocon/upstream"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func BenchmarkRewriteHost(b *testing.B) {
	originalReq := fasthttp.AcquireRequest()
	req := fasthttp.AcquireRequest()
	originalReq.SetHost("example.com.ccnproxy:3000")

	for n := 0; n < b.N; n++ {
		rewriteHost(req, originalReq)
	}
}

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

type testServer struct {
	host  string
	port  int
	https bool
	h     http.HandlerFunc
}

func (s *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.h(w, r)
}

// Sets up a client, a proxy, servers, and connections among them.
// Then `fn` is called with the client whose requests are routed to the proxy.
func testProxy(
	t *testing.T,
	servers []*testServer,
	fn func(client *http.Client),
) {
	// This is used for client-proxy connection.
	ln := fasthttputil.NewInmemoryListener()

	// These are used for proxy-server connections.
	lns := make([]*fasthttputil.InmemoryListener, 0, len(servers))
	for range servers {
		lns = append(lns, fasthttputil.NewInmemoryListener())
	}

	// Requests from this are forwarded to proxy.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		},
		Timeout: time.Second * 3,
	}

	proxy := New(&fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			// Find the matching server.
			for i, ln := range lns {
				if fmt.Sprintf("%s:%d", servers[i].host, servers[i].port) == addr {
					return ln.Dial()
				}
			}

			// This should never get called.
			panic(fmt.Sprintf("invalid dialing: %s", addr))
		},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}, "", zap.NewNop(), &upstream.Upstream{})

	// Get the servers running.
	for i := range servers {
		go func(i int) {
			var err error

			if servers[i].https {
				err = http.ServeTLS(lns[i], servers[i], "../example.cert", "../example.key")
			} else {
				err = http.Serve(lns[i], servers[i])
			}

			if err != nil {
				t.Fatal(err)
			}
		}(i)
	}

	// Get the proxy running.
	go func() {
		err := fasthttp.Serve(ln, proxy.Handler)

		if err != nil {
			t.Fatal(err)
		}
	}()

	fn(client)
}

func TestProxyGET(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/", r.URL.Path)
		w.Header().Set("some-key", "some-value")
		w.Write([]byte("OK"))
	}

	testProxy(
		t,
		[]*testServer{
			{"foo.com", 3000, false, handler},
			{"bar.com", 443, true, handler},
		},
		func(client *http.Client) {
			f := func(host string) {
				req, err := http.NewRequest("GET", "http://...", nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Host = host
				res, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resBody, err := ioutil.ReadAll(res.Body)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, "OK", string(resBody), "correct response body should be returned")
				assert.Equal(t, 200, res.StatusCode, "status code should be 200")
				assert.Equal(t, "some-value", res.Header.Get("some-key"), "header values should be passed from server to client")
				assert.NotZero(t, res.Header.Get("x-chocon-id"), "x-chocon-id header value should be set")
			}

			t.Run("Serial", func(t *testing.T) {
				f("foo.com.ccnproxy:3000")
				f("bar.com.ccnproxy-secure")
			})

			t.Run("Concurrent", func(t *testing.T) {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(2)
					go func() {
						defer wg.Done()
						f("bar.com.ccnproxy-secure")
					}()
					go func() {
						defer wg.Done()
						f("foo.com.ccnproxy:3000")
					}()
				}
				wg.Wait()
			})
		},
	)
}

func TestProxyPOST(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/some-path", r.URL.Path, "the request path should be passed from client to server")
		assert.Equal(t, "some-value", r.Header.Get("some-key"), "header values should be passed from client to server")
		assert.Equal(t, "foo", r.URL.Query().Get("a"), "query parameters should be passed from client to server")
		w.WriteHeader(201)
		_, err := io.Copy(w, r.Body)
		if err != nil {
			t.Fatal(err)
		}
	}

	testProxy(
		t,
		[]*testServer{
			{"foo.com", 3000, false, handler},
			{"bar.com", 443, true, handler},
		},
		func(client *http.Client) {
			f := func(host string) {
				someLongString := strings.Repeat(time.Now().Format("2006-01-02T15:04:05"), 100)
				req, err := http.NewRequest("POST", "http://.../some-path?a=foo", bytes.NewBufferString(someLongString))
				req.Header.Set("some-key", "some-value")
				if err != nil {
					t.Fatal(err)
				}
				req.Host = host
				res, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resBody, err := ioutil.ReadAll(res.Body)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, someLongString, string(resBody))
				assert.Equal(t, 201, res.StatusCode)
			}

			t.Run("Serial", func(t *testing.T) {
				f("foo.com.ccnproxy:3000")
				f("bar.com.ccnproxy-secure")
			})

			t.Run("Concurrent", func(t *testing.T) {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(2)
					go func() {
						defer wg.Done()
						f("foo.com.ccnproxy:3000")
					}()
					go func() {
						defer wg.Done()
						f("bar.com.ccnproxy-secure")
					}()
				}
				wg.Wait()
			})
		},
	)
}

func TestProxyDELETE(t *testing.T) {
	testProxy(
		t,
		[]*testServer{
			{
				"foo.com", 3000, false,
				func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "DELETE", r.Method)
					_, err := w.Write([]byte("OK"))
					if err != nil {
						t.Fatal(err)
					}
				},
			},
		},
		func(client *http.Client) {
			req, err := http.NewRequest("DELETE", "http://...", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = "foo.com.ccnproxy:3000"
			res, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, "OK", string(resBody))
		},
	)
}
