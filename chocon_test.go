package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
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

// echoServer implements http.Handler.
type echoServer struct {
	host  string
	port  uint
	https bool
	// Stores the most recent request.
	lastRequest *http.Request
	mu          *sync.Mutex
}

// The output will be like 'foo.ccnproxy', 'foo.ccnproxy:3000' or 'foo.ccnproxy-secure.'
// Chocon expects them to be set in the host header.
func (s *echoServer) hostForChocon() string {
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

func (s *echoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)

	time.Sleep(time.Millisecond * 10)

	s.mu.Lock()
	s.lastRequest = r
	s.mu.Unlock()

	if r.Method == "GET" {
		fmt.Fprintf(w, "response from %s", s.host)
		return
	}

	// Echoes back the content when the server has received a non-GET request.
	if _, err := io.Copy(w, r.Body); err != nil {
		w.WriteHeader(400)
		return
	}
}

var servers = []*echoServer{
	{"foo", 80, false, nil, &sync.Mutex{}},
	{"bar", 3000, false, nil, &sync.Mutex{}},
	{"baz", 443, true, nil, &sync.Mutex{}},
}

// This sets up the client, the proxy, the echo server, and the connections among them.
// After that, the request is sent to chocon and its response is returned.
func sendRequestViaChocon(t *testing.T, req *http.Request) *http.Response {
	// This is used for client-proxy connection.
	ln := fasthttputil.NewInmemoryListener()

	// These are used for proxy-server connections.
	lns := make(map[*echoServer]*fasthttputil.InmemoryListener)
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

			t.Fatal("invalid dialing")

			// This never gets called.
			return nil, nil
		},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}, "", zap.NewNop())

	// Get the servers running.
	for _, server := range servers {
		go func(s *echoServer) {
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

	res, err := client.Do(req)

	if err != nil {
		t.Fatal(err)
	}

	return res
}

// See if requests are directed to correct target servers.
func TestChoconHostSelection(t *testing.T) {
	for _, server := range servers {
		req, _ := http.NewRequest("GET", "http://chocon", nil)

		req.Host = server.hostForChocon()

		res := sendRequestViaChocon(t, req)

		body, err := ioutil.ReadAll(res.Body)

		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, res.StatusCode, 200)
		assert.Equal(t, string(body), fmt.Sprintf("response from %s", server.host))
	}
}

// Sends a POST request.
func TestChoconPost(t *testing.T) {
	server := servers[0]
	req, _ := http.NewRequest("POST", "http://chocon", bytes.NewBufferString("hello"))

	req.Host = server.hostForChocon()

	res := sendRequestViaChocon(t, req)

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, res.StatusCode, 200)
	assert.Equal(t, string(body), "hello")
}

// Sends a PUT request.
func TestChoconPut(t *testing.T) {
	server := servers[0]
	req, _ := http.NewRequest("PUT", "http://chocon", bytes.NewBufferString("hello"))

	req.Host = server.hostForChocon()

	res := sendRequestViaChocon(t, req)

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, res.StatusCode, 200)
	assert.Equal(t, string(body), "hello")
}

// Sends a GET request.
func TestChoconGet(t *testing.T) {
	server := servers[0]
	req, _ := http.NewRequest("GET", "http://chocon", nil)
	req.Host = server.hostForChocon()

	res := sendRequestViaChocon(t, req)

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, res.StatusCode, 200)
	assert.Equal(t, string(body), fmt.Sprintf("response from %s", server.host))
}

// See if the original request's header values are passed to the target server.
func TestChoconHeader(t *testing.T) {
	server := servers[0]
	req, _ := http.NewRequest("GET", "http://chocon", nil)
	req.Host = server.hostForChocon()
	req.Header.Set("some-key", "some-value")

	res := sendRequestViaChocon(t, req)

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, res.StatusCode, 200)
	assert.Equal(t, string(body), fmt.Sprintf("response from %s", server.host))
	assert.Equal(t, server.lastRequest.Header.Get("some-key"), "some-value")
}

// We have to be careful about race conditions when fasthttp is used.
// Test this with the '-race' option to check it.
func TestChoconHeavyLoad(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		for _, server := range servers {
			wg.Add(1)

			go func(server *echoServer) {
				defer wg.Done()

				req, _ := http.NewRequest("GET", "http://chocon", nil)
				req.Host = server.hostForChocon()
				req.Header.Set("some-key", "some-value")

				res := sendRequestViaChocon(t, req)

				assert.Equal(t, res.StatusCode, 200)

			}(server)
		}
	}

	wg.Wait()
}
