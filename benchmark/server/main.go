package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.Copy(w, req.Body)
	})

	go func() {
		err := http.ListenAndServe(":80", nil)

		if err != nil {
			log.Fatal("ListenAndServe", err)
		}
	}()

	err := http.ListenAndServeTLS(":443", "server.crt", "server.key", nil)

	if err != nil {
		log.Fatal("ListenAndServeTLS", err)
	}
}
