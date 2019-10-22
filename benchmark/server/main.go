package main

import (
	"log"
	"os"

	"github.com/valyala/fasthttp"
)

func main() {
	s := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			ctx.SetContentType("text/plain")
			ctx.Write(ctx.PostBody())
		},
	}

	useHTTPS := os.Getenv("HTTPS") != ""

	if useHTTPS {
		if err := s.ListenAndServeTLS(":443", "server.crt", "server.key"); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := s.ListenAndServe(":80"); err != nil {
			log.Fatal(err)
		}
	}
}
