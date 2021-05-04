package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/korylprince/printer-manager-cups/control"
)

func DoCommand(pkt *control.Packet) {
	resp, err := control.Do(pkt)
	if err != nil {
		if strings.Contains(err.Error(), "connect: no such file or directory") {
			fmt.Println("Control socket not found. Are you sure the server is running?")
		} else {
			fmt.Println("Unable to send command to server:", err)
		}
		os.Exit(1)
	}
	fmt.Println(resp.Message)
}

func usage() {
	fmt.Printf("Usage: %s [command]:\nCommands:\n\tsync [usernames...]\tsyncs printers, optionally including usernames\n\tclear-cache\t\tclears printer cache\n\tlist-drivers\t\tlists drivers found by CUPS\n", os.Args[0])
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "sync":
		if len(os.Args) > 2 {
			users := os.Args[2:len(os.Args)]
			b, err := json.Marshal(users)
			if err != nil {
				fmt.Println("Unable to marshal users:", err)
				os.Exit(1)
			}
			fmt.Print("Server returned: ")
			DoCommand(&control.Packet{Type: control.PacketTypeSync, Message: string(b)})
		} else {
			fmt.Print("Server returned: ")
			DoCommand(&control.Packet{Type: control.PacketTypeSync})
		}
	case "clear-cache":
		fmt.Print("Server returned: ")
		DoCommand(&control.Packet{Type: control.PacketTypeClearCache})
		fmt.Println("You will probably want to run the sync command now")
	case "list-drivers":
		fmt.Println("Server returned:")
		DoCommand(&control.Packet{Type: control.PacketTypeListDrivers})
	default:
		fmt.Println("Unknown command:", os.Args[1])
		usage()
	}
}
