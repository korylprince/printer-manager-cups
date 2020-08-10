package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/korylprince/printer-manager-cups/control"
)

func DoCommand(pt control.PacketType) {
	resp, err := control.Do(&control.Packet{Type: pt})
	if err != nil {
		if strings.Contains(err.Error(), "connect: no such file or directory") {
			fmt.Println("Control socket not found. Are you sure the server is running?")
		} else {
			fmt.Println("Unable to send command to server:", err)
		}
		os.Exit(1)
	}

	fmt.Println("Server returned:", resp.Message)
}

func usage() {
	fmt.Printf("Usage: %s [command]:\nCommands:\n\tsync\t\tsyncs printers\n\tclear-cache\tclears printer cache\n", os.Args[0])
	os.Exit(1)
}

func main() {
	if len(os.Args) != 2 {
		usage()
	}
	switch os.Args[1] {
	case "sync":
		DoCommand(control.PacketTypeSync)
	case "clear-cache":
		DoCommand(control.PacketTypeClearCache)
		fmt.Println("You will probably want to run the sync command now")
	default:
		fmt.Println("Unknown command:", os.Args[1])
		usage()
	}
}
