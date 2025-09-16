package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/faroedev/faroe"
)

type serverStruct struct {
	server *faroe.ServerStruct
}

func (server *serverStruct) listen(port string) error {
	log.Printf("Listening on port %s...", port)
	return http.ListenAndServe(fmt.Sprintf(":%s", port), http.HandlerFunc(server.handle))
}

func (server *serverStruct) handle(w http.ResponseWriter, r *http.Request) {
	// TODO check CORS
	if r.Method == "OPTIONS" && r.URL.Path == "/" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == "POST" && r.URL.Path == "/" {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resultJSON, err := server.server.ResolveActionInvocationEndpointRequestWithBlocklist(string(bodyBytes), nil)
		if err != nil {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resultJSON))
		return
	}

	w.WriteHeader(http.StatusNotFound)
}
