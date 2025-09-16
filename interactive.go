package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type InteractiveShell struct {
	storage *storageStruct
	server  *serverStruct
	port    string
	reader  *bufio.Reader
}

func NewInteractiveShell(storage *storageStruct, server *serverStruct, port string) *InteractiveShell {
	return &InteractiveShell{
		storage: storage,
		server:  server,
		port:    port,
		reader:  bufio.NewReader(os.Stdin),
	}
}

func (shell *InteractiveShell) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  reset - Clear all data from storage")
	fmt.Println("  help  - Show this help message")
	fmt.Println("  exit  - Exit program")
}

func (shell *InteractiveShell) Run() {
	// Start server in a goroutine
	go shell.server.listen(shell.port)

	fmt.Printf("Interactive mode started. Server running on port %s\n", shell.port)
	fmt.Println("Type 'help' for available commands.")
	fmt.Print("> ")

	for {
		line, err := shell.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nEOF received, exiting...")
				return
			}
			log.Printf("Error reading from stdin: %v", err)
			continue
		}

		command := strings.TrimSpace(line)
		shell.handleCommand(command)
	}
}

func (shell *InteractiveShell) handleCommand(command string) {
	switch command {
	case "reset":
		err := shell.storage.Clear()
		if err != nil {
			fmt.Printf("Error clearing storage: %v\n", err)
		} else {
			fmt.Println("Storage cleared successfully")
		}
	case "help":
		shell.showHelp()
	case "exit", "quit":
		fmt.Println("Exiting...")
		os.Exit(0)
	case "":
		// Empty command, just show prompt again
	default:
		fmt.Printf("Unknown command: %s (type 'help' for available commands)\n", command)
	}

	fmt.Print("> ")
}
