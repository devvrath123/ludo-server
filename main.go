package main

import (
	"log"
	"net/http"
)

var hub = newHub()

func main() {
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	log.Println("listening on 127.0.0.1:8080")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
