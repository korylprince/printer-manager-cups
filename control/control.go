//go:generate stringer -type PacketType

package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

type PacketType int

//SearchPaths will be searched for a valid directory to place the control socket
var SearchPaths = []string{"/var/run", "/run"}

const (
	PacketTypeSync PacketType = iota
	PacketTypeResponse
)

//Packet represents a control packet
type Packet struct {
	Type    PacketType `json:"type"`
	Message string     `json:"msg"`
}

//GetSocket returns a control socket path, or an error if none exists
func GetSocket() (string, error) {
	for _, path := range SearchPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return fmt.Sprintf("%s/printer-manager.sock", path), nil
		}
	}
	return "", errors.New("Unable to find valid run directory")
}

//Listener listens for control Packets and runs registered handlers on them
type Listener struct {
	Socket     string
	listener   net.Listener
	handlers   map[PacketType]func(p *Packet) *Packet
	handlersMu *sync.RWMutex
}

func (l *Listener) worker() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			log.Println("WARN: Unable to accept control message:", err)
			continue
		}

		d := json.NewDecoder(conn)
		p := new(Packet)
		if err = d.Decode(p); err != nil {
			conn.Close()
			log.Println("WARN: Unable to decode control message:", err)
			continue
		}

		l.handlersMu.RLock()
		f, ok := l.handlers[p.Type]
		l.handlersMu.RUnlock()
		if !ok || f == nil {
			conn.Close()
			log.Printf("WARN: Unregistered handler for PacketType: %s\n", p.Type.String())
			continue
		}

		log.Printf("INFO: Running handler for control message: %s\n", p.Type.String())

		go func() {
			resp := f(p)
			e := json.NewEncoder(conn)
			if err = e.Encode(resp); err != nil {
				conn.Close()
				return
			}

			if err = conn.Close(); err != nil {
				log.Println("WARN: Error closing conn:", err)
			}
		}()
	}
}

//New returns a new Listener or an error if one occurred
func New() (*Listener, error) {
	sock, err := GetSocket()
	if err != nil {
		return nil, fmt.Errorf("Unable to get socket: %v", err)
	}

	if err := os.Remove(sock); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to remove old socket: %v", err)
	}

	l, err := net.Listen("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("Unable to listen on %s: %v", sock, err)
	}

	if err = os.Chmod(sock, 0777); err != nil {
		return nil, fmt.Errorf("Unable to set permissions for socket: %v", err)
	}

	lis := &Listener{Socket: sock,
		listener:   l,
		handlers:   make(map[PacketType]func(*Packet) *Packet),
		handlersMu: new(sync.RWMutex),
	}
	go lis.worker()

	return lis, nil
}

//Register registers the given handler for the given Packet type
func (l *Listener) Register(t PacketType, handler func(p *Packet) *Packet) {
	l.handlersMu.Lock()
	l.handlers[t] = handler
	l.handlersMu.Unlock()
}

//Do sends the given control packet and returns the response, or an error if one occurred
func Do(p *Packet) (*Packet, error) {
	sock, err := GetSocket()
	if err != nil {
		return nil, fmt.Errorf("Unable to get socket: %v", err)
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("Unable to dial %s: %v", sock, err)
	}

	e := json.NewEncoder(conn)
	if err = e.Encode(p); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Unable to encode packet: %v", err)
	}

	resp := new(Packet)
	d := json.NewDecoder(conn)
	if err = d.Decode(resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Unable to decode response: %v", err)
	}

	if err = conn.Close(); err != nil {
		return nil, fmt.Errorf("Error closing conn: %v", err)
	}

	return resp, nil
}
