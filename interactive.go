package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type InteractiveShell struct {
	reader  *bufio.Reader
	storage *storageStruct
	errChan chan error
}

func NewInteractiveShell(storage *storageStruct) *InteractiveShell {
	return &InteractiveShell{
		storage: storage,
		reader:  bufio.NewReader(os.Stdin),
	}
}

func (shell *InteractiveShell) listen() {
	fmt.Println("Interactive mode started.")
	fmt.Println("Type 'help' for available commands.")
	fmt.Print("> ")

	errChan := make(chan error, 1)

	go func() {
		for {
			line, err := shell.reader.ReadString('\n')
			if err != nil {
				errChan <- err
				return
			}
			command := strings.TrimSpace(line)
			shell.handleCommand(command)
		}
	}()

	shell.errChan = errChan
}

func (shell *InteractiveShell) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  reset - Clear all data from storage")
	fmt.Println("  help  - Show this help message")
	fmt.Println("  exit  - Exit program")
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
