package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

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

// mockServer implements http.Handler.
type mockServer struct {
	host  string
	port  uint
	https bool
	// Stores the most recent request.
	lastRequest     *http.Request
	lastRequestLock *sync.Mutex
	// Status codes for every response is set to this value.
	statusCode int
}

// The return value of this function is written to response bodies.
func (s *mockServer) response(method string, reqBody []byte) []byte {
	if method == "GET" {
		return []byte(fmt.Sprintf("hello from %s", s.host))
	}

	// For non-GET requests, the request body is echoed back to the client.
	return reqBody
}

// The output of this function will be like 'foo.com.ccnproxy', 'foo.com.ccnproxy:3000' or 'foo.com.ccnproxy-ssl'.
// The proxy expects them to be set in the host header for selecting the target server.
func (s *mockServer) hostForProxy() string {
	host := s.host

	if s.https {
		host += ".ccnproxy-ssl"

		if s.port != 443 {
			host += fmt.Sprintf(":%d", s.port)
		}
	} else {
		host += ".ccnproxy"

		if s.port != 80 {
			host += fmt.Sprintf(":%d", s.port)
		}
	}

	return host
}

func (s *mockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(s.statusCode)

	time.Sleep(time.Millisecond * 10)

	s.lastRequestLock.Lock()
	s.lastRequest = r
	s.lastRequestLock.Unlock()

	reqBody, err := ioutil.ReadAll(r.Body)

	if err != nil {
		w.WriteHeader(400)
		return
	}

	if _, err := w.Write(s.response(r.Method, reqBody)); err != nil {
		w.WriteHeader(400)
		return
	}
}

var servers = []*mockServer{
	{"foo.com", 80, false, nil, &sync.Mutex{}, 200},
	{"bar.com", 3000, false, nil, &sync.Mutex{}, 200},
	{"baz.com", 443, true, nil, &sync.Mutex{}, 200},
	{"qux.com", 80, false, nil, &sync.Mutex{}, 404},
}

// Sets up a client, a proxy, an echo server, and connections among them.
// Then `fn` is called with the client.
func testProxy(t *testing.T, fn func(t *testing.T, client *http.Client)) {
	// This is used for client-proxy connection.
	ln := fasthttputil.NewInmemoryListener()

	// These are used for proxy-server connections.
	lns := make(map[*mockServer]*fasthttputil.InmemoryListener)
	for _, server := range servers {
		lns[server] = fasthttputil.NewInmemoryListener()
	}

	// Requests from this are forwarded to chocon.
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
			for k, v := range lns {
				if fmt.Sprintf("%s:%d", k.host, k.port) == addr {
					return v.Dial()
				}
			}

			// This never gets called.
			panic("invalid dialing")
		},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}, "", zap.NewNop())

	// Get the servers running.
	for _, server := range servers {
		go func(s *mockServer) {
			var err error

			if s.https {
				err = http.ServeTLS(lns[s], s, "../example.cert", "../example.key")
			} else {
				err = http.Serve(lns[s], s)
			}

			if err != nil {
				t.Fatal(err)
			}
		}(server)
	}
	// Get the proxy running.
	go func() {
		err := fasthttp.Serve(ln, proxy.Handler)

		if err != nil {
			t.Fatal(err)
		}
	}()

	fn(t, client)
}

func testProxyOneRequest(t *testing.T, client *http.Client, target *mockServer, method string, reqBody []byte) {
	req, err := http.NewRequest(method, "http://...", bytes.NewBuffer(reqBody))
	req.Header.Add("some-key", "some-value")
	if err != nil {
		t.Fatal(err)
	}
	req.Host = target.hostForProxy()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, res.StatusCode, target.statusCode, "should return the correct status code")
	assert.ElementsMatch(t, resBody, target.response(method, reqBody), "the response body should be correct")
	assert.Equal(t, target.lastRequest.Header.Get("some-key"), "some-value", "header values should have been passed to the target")
	assert.NotZero(t, res.Header.Get("x-chocon-id"), "x-chocon-id header value should be set")
}

func TestProxyGETSingle(t *testing.T) {
	testProxy(t, func(t *testing.T, client *http.Client) {
		testProxyOneRequest(t, client, servers[0], "GET", nil)
	})
}

func TestProxyPOSTSingle(t *testing.T) {
	testProxy(t, func(t *testing.T, client *http.Client) {
		testProxyOneRequest(t, client, servers[0], "POST", []byte("request"))
	})
}

func TestProxyPostConcurrentLarge(t *testing.T) {
	reqBody := ""
	for i := 0; i < 100000; i++ {
		reqBody += "a"
	}
	testProxy(t, func(t *testing.T, client *http.Client) {
		testProxyOneRequest(t, client, servers[0], "POST", []byte(reqBody))
	})
}

func TestProxyPOSTSerial(t *testing.T) {
	testProxy(t, func(t *testing.T, client *http.Client) {
		for i, server := range servers {
			for j := 0; j < 10; j++ {
				testProxyOneRequest(t, client, server, "POST", []byte(fmt.Sprintf("request %d %d", i, j)))
			}
		}
	})
}

func TestProxyGETSerial(t *testing.T) {
	testProxy(t, func(t *testing.T, client *http.Client) {
		for _, server := range servers {
			for j := 0; j < 10; j++ {
				testProxyOneRequest(t, client, server, "GET", nil)
			}
		}
	})
}

func TestProxyPOSTConcurrent(t *testing.T) {
	testProxy(t, func(t *testing.T, client *http.Client) {
		var wg sync.WaitGroup
		for i, server := range servers {
			for j := 0; j < 100; j++ {
				wg.Add(1)
				go func(i, j int, server *mockServer) {
					defer wg.Done()
					testProxyOneRequest(t, client, server, "POST", []byte(fmt.Sprintf("request %d %d", i, j)))
				}(i, j, server)
			}
		}
		wg.Wait()
	})
}
