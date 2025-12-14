package tiauth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type interactiveShell struct {
	reader  *bufio.Reader
	storage *storageStruct
	errChan chan error
}

func newInteractiveShell(storage *storageStruct) *interactiveShell {
	return &interactiveShell{
		storage: storage,
		reader:  bufio.NewReader(os.Stdin),
	}
}

func (shell *interactiveShell) listen() {
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

func (shell *interactiveShell) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  reset - Clear all data from storage")
	fmt.Println("  help  - Show this help message")
	fmt.Println("  exit  - Exit program")
}

func (shell *interactiveShell) handleCommand(command string) {
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
