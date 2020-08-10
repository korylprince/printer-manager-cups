package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/korylprince/printer-manager-cups/control"
)

func main() {
	c := new(Config)
	if err := envconfig.Process("", c); err != nil {
		log.Fatalln("ERROR: Unable to process configuration:", err)
	}

	con, err := control.New()
	if err != nil {
		log.Fatalln("ERROR: Unable to set up control socket:", err)
	}

	inputSync := make(chan struct{})
	inputClearCache := make(chan struct{})
	output := make(chan string)

	con.Register(control.PacketTypeSync, func(p *control.Packet) *control.Packet {
		inputSync <- struct{}{}
		return &control.Packet{Type: control.PacketTypeResponse, Message: <-output}
	})

	con.Register(control.PacketTypeClearCache, func(p *control.Packet) *control.Packet {
		inputClearCache <- struct{}{}
		return &control.Packet{Type: control.PacketTypeResponse, Message: <-output}
	})

	log.Println("INFO: Listening for commands on", con.Socket)

	t := time.NewTimer(0)

	for {
		select {
		case <-inputSync:
			log.Println("INFO: Sync command received. Running sync")
			if err := Sync(c); err != nil {
				log.Println("WARN: Sync failed:", err)
				output <- fmt.Sprintf("Sync failed: %v", err)
				break
			}
			output <- "Sync completed successfully"
		case <-inputClearCache:
			log.Println("INFO: ClearCache command received. Clearing cache")
			if err := ClearCache(c); err != nil {
				log.Println("WARN: Clearing cache failed:", err)
				output <- fmt.Sprintf("Clearing cache failed: %v", err)
				break
			}
			output <- "Cache cleared successfully"
		case <-t.C:
			if err := Sync(c); err != nil {
				log.Println("WARN: Sync failed:", err)
			}
		}

		t.Stop()
		t.Reset(c.SyncInterval)
	}
}
