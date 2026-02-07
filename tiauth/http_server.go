package tiauth

import (
	"encoding/json"
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

// commandServer handles management commands on a separate 127.0.0.2 listener.
type commandServer struct {
	storage *storageStruct
	errChan chan error
}

func (cs *commandServer) listen(port string) {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)
		addr := fmt.Sprintf("127.0.0.2:%s", port)
		log.Printf("Command listener on %s", addr)
		err := http.ListenAndServe(addr, http.HandlerFunc(cs.handle))
		if err != nil {
			errChan <- err
		}
	}()

	cs.errChan = errChan
}

func (cs *commandServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && r.URL.Path == "/command" {
		cs.handleCommand(w, r)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (cs *commandServer) handleCommand(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var body struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid json"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch body.Command {
	case "reset":
		log.Printf("[%s] command=reset\n", time.Now().Format("15:04:05.000"))
		err := cs.storage.Clear()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		} else {
			w.Write([]byte(`{"success":true,"message":"Storage cleared"}`))
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"unknown command: %s"}`, body.Command)
	}
}
