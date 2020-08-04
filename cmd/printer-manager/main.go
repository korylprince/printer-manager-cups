package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/korylprince/printer-manager-cups/control"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "sync" {
		fmt.Printf("Usage: %s [command]:\nCommands:\n\tsync\tsyncs printers\n", os.Args[0])
		os.Exit(1)
	}

	resp, err := control.Do(&control.Packet{Type: control.PacketTypeSync})
	if err != nil {
		if strings.Contains(err.Error(), "connect: no such file or directory") {
			fmt.Println("Control socket not found. Are you sure the server is running?")
		} else {
			fmt.Println("Unable to sync:", err)
		}
		os.Exit(1)
	}

	fmt.Println("Server returned:", resp.Message)
}
