package tiauth

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/faroedev/faroe"
)

type httpServer struct {
	server          *faroe.ServerStruct
	storage         *storageStruct
	enableReset     bool
	corsAllowOrigin string
	errChan         chan error
}

func (server *httpServer) listen(port string) {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)
		log.Printf("Listening on port %s...", port)
		err := http.ListenAndServe(fmt.Sprintf(":%s", port), http.HandlerFunc(server.handle))
		if err != nil {
			errChan <- err
		}
	}()

	server.errChan = errChan
}

func (server *httpServer) handle(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers only if origin is configured
	if server.corsAllowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", server.corsAllowOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}

	// Handle OPTIONS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case r.Method == "POST" && r.URL.Path == "/":
		server.handleInvoke(w, r)
	case r.Method == "GET" && r.URL.Path == "/alive":
		server.handleAlive(w)
	case server.enableReset && r.Method == "POST" && r.URL.Path == "/reset":
		server.handleReset(w)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (server *httpServer) handleInvoke(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resultJSON, err := server.server.ResolveActionInvocationEndpointRequestWithBlocklist(string(bodyBytes), nil)
	if err != nil {
		log.Printf("[%s] invoke action error=%v\n", time.Now().Format("15:04:05.000"), err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(resultJSON))
}

func (server *httpServer) handleAlive(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"alive"}`))
}

func (server *httpServer) handleReset(w http.ResponseWriter) {
	log.Printf("[%s] request=%s\n", time.Now().Format("15:04:05.000"), "reset")
	server.storage.Clear()
}
