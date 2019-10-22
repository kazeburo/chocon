package stats

import (
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// Set up a server, a client and a connection between them.
// Then, `fn` is called which is supposed to be the main part of the test.
// The http client and the metrics object can be accessed inside `fn`
func testWithMockServer(
	t *testing.T,
	handler fasthttp.RequestHandler,
	fn func(*testing.T, *http.Client, *Metrics),
) {
	mw, err := New()

	if err != nil {
		t.Fatal(err)
	}

	ln := fasthttputil.NewInmemoryListener()

	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return ln.Dial()
			},
		},
	}

	server := fasthttp.Server{
		Handler:     mw.WrapHandler(handler),
		ReadTimeout: time.Second,
	}

	go server.Serve(ln)
	defer server.Shutdown()

	fn(t, &client, mw)
}

func TestWithMockServerSimple(t *testing.T) {
	testWithMockServer(t,
		func(ctx *fasthttp.RequestCtx) {
			ctx.Write([]byte("Hello"))
		},
		func(t *testing.T, client *http.Client, mw *Metrics) {
			_, err := client.Get("http://example.com")

			if err != nil {
				t.Fatal((err))
			}
			d, err := mw.Data()
			if err != nil {
				t.Fatal(err)
			}
			if d.Request.Count != 1 {
				t.Errorf("Count: expected 1 but acutual %d", d.Request.Count)
			}
			if d.Request.StatusCount[200] != 1 {
				t.Errorf("StatusCount[200]: expected 1 but acutual %d", d.Request.StatusCount[200])
			}
		},
	)
}

func TestWithMockServer500(t *testing.T) {
	testWithMockServer(t,
		func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(500)
			ctx.Write([]byte("Hello"))
		},
		func(t *testing.T, client *http.Client, mw *Metrics) {
			_, err := client.Get("http://example.com")

			if err != nil {
				t.Fatal((err))
			}
			d, err := mw.Data()
			if err != nil {
				t.Fatal(err)
			}
			if d.Request.Count != 1 {
				t.Errorf("Count: expected 1 but acutual %d", d.Request.Count)
			}
			if d.Request.StatusCount[500] != 1 {
				t.Errorf("StatusCount[500]: expected 1 but acutual %d", d.Request.StatusCount[500])
			}
		},
	)
}

func TestWithMockServerConcurrent(t *testing.T) {
	testWithMockServer(
		t,
		func(ctx *fasthttp.RequestCtx) {
			ctx.Write([]byte("Hello"))
		},
		func(t *testing.T, client *http.Client, mw *Metrics) {
			wg := sync.WaitGroup{}

			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := client.Get("http://example.com")
					if err != nil {
						t.Fatal(err)
					}
				}()
			}
			wg.Wait()
			d, err := mw.Data()
			if err != nil {
				t.Fatal(err)
			}
			if d.Request.Count != 100 {
				t.Errorf("Count: expected 100 but acutual %d", d.Request.Count)
			}
			if d.Request.StatusCount[200] != 100 {
				t.Errorf("StatusCount[200]: expected 100 but acutual %d", d.Request.StatusCount[200])
			}
		},
	)
}
