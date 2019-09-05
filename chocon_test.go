package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/kazeburo/chocon/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
	"go.uber.org/zap"
)

// testServer implements http.Handler.
type testServer struct {
	host  string
	port  uint
	https bool
	// Stores the most recent request.
	lastRequest     *http.Request
	lastRequestLock *sync.Mutex
	statusCode      int
}

func (s *testServer) response(method string, reqBody []byte) []byte {
	if method == "GET" {
		return []byte(fmt.Sprintf("hello from %s", s.host))
	}

	return reqBody
}

// The output will be like 'foo.ccnproxy', 'foo.ccnproxy:3000' or 'foo.ccnproxy-secure.'
// Chocon expects them to be set in the host header for selecting the target server.
func (s *testServer) hostForChocon() string {
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

func (s *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

var servers = []*testServer{
	{"foo", 80, false, nil, &sync.Mutex{}, 200},
	{"bar", 3000, false, nil, &sync.Mutex{}, 200},
	{"baz", 443, true, nil, &sync.Mutex{}, 200},
	{"qux", 80, false, nil, &sync.Mutex{}, 404},
}

// Sets up a client, a proxy(chocon), an echo server, and connections among them.
// Then `f` is called with the client.
func testChocon(t *testing.T, f func(t *testing.T, client *http.Client)) {
	// This is used for client-proxy connection.
	ln := fasthttputil.NewInmemoryListener()

	// These are used for proxy-server connections.
	lns := make(map[*testServer]*fasthttputil.InmemoryListener)
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

	proxy := proxy.New(&fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			// Find the matching server.
			for k, v := range lns {
				if fmt.Sprintf("%s:%d", k.host, k.port) == addr {
					return v.Dial()
				}
			}

			log.Fatal("invalid dialing")

			// This never gets called.
			return nil, nil
		},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}, "", zap.NewNop())

	// Get the servers running.
	for _, server := range servers {
		go func(s *testServer) {
			var err error

			if s.https {
				err = http.ServeTLS(lns[s], s, "./example.cert", "./example.key")
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

	f(t, client)
}

func testChoconOneRequest(t *testing.T, client *http.Client, target *testServer, method string, reqBody []byte) {
	req, err := http.NewRequest(method, "http://...", bytes.NewBuffer(reqBody))
	req.Header.Add("some-key", "some-value")
	if err != nil {
		t.Fatal(err)
	}
	req.Host = target.hostForChocon()
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

func TestChoconGETSingle(t *testing.T) {
	testChocon(t, func(t *testing.T, client *http.Client) {
		testChoconOneRequest(t, client, servers[0], "GET", nil)
	})
}

func TestChoconPOSTSingle(t *testing.T) {
	testChocon(t, func(t *testing.T, client *http.Client) {
		testChoconOneRequest(t, client, servers[0], "POST", []byte("request"))
	})
}

func TestChoconPostConcurrentLarge(t *testing.T) {
	reqBody := ""
	for i := 0; i < 100000; i++ {
		reqBody += "a"
	}
	testChocon(t, func(t *testing.T, client *http.Client) {
		testChoconOneRequest(t, client, servers[0], "POST", []byte(reqBody))
	})
}

func TestChoconPOSTSerial(t *testing.T) {
	testChocon(t, func(t *testing.T, client *http.Client) {
		for i, server := range servers {
			for j := 0; j < 10; j++ {
				testChoconOneRequest(t, client, server, "POST", []byte(fmt.Sprintf("request %d %d", i, j)))
			}
		}
	})
}

func TestChoconGETSerial(t *testing.T) {
	testChocon(t, func(t *testing.T, client *http.Client) {
		for _, server := range servers {
			for j := 0; j < 10; j++ {
				testChoconOneRequest(t, client, server, "GET", nil)
			}
		}
	})
}

func TestChoconPOSTConcurrent(t *testing.T) {
	testChocon(t, func(t *testing.T, client *http.Client) {
		var wg sync.WaitGroup
		for i, server := range servers {
			for j := 0; j < 100; j++ {
				wg.Add(1)
				go func(i, j int, server *testServer) {
					defer wg.Done()
					testChoconOneRequest(t, client, server, "POST", []byte(fmt.Sprintf("request %d %d", i, j)))
				}(i, j, server)
			}
		}
		wg.Wait()
	})
}
