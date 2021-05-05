package user

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
)

//UtmpLocations specifies where to search for utmp files
var UtmpLocations = []string{"/var/run/utmp", "/run/utmp"}

//UtmpxLocations specifies where to search for utmpx files
var UtmpxLocations = []string{"/var/run/utmpx"}

const typeUser = 7

type utmp struct {
	Type int16
	_    [42]byte
	Name [32]byte
	_    [308]byte
}

type utmpx struct {
	Name [256]byte
	_    [40]byte
	Type int16
	_    [330]byte
}

func coalesce(items []string) []string {
	set := make(map[string]struct{})
	for _, i := range items {
		set[i] = struct{}{}
	}

	coalesced := make([]string, 0, len(set))
	for i := range set {
		coalesced = append(coalesced, i)
	}

	return coalesced
}

func readUtmp(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("Unable to open path %s: %w", path, err)
	}
	defer f.Close()

	var usernames []string
	for {
		u := new(utmp)
		if err := binary.Read(f, binary.LittleEndian, u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("Unable to read: %w", err)
		}
		if u.Type == typeUser {
			end := bytes.IndexByte(u.Name[:], 0)
			usernames = append(usernames, string(u.Name[:end]))
		}
	}

	return usernames, nil
}

func readUtmpx(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("Unable to open path %s: %w", path, err)
	}
	defer f.Close()

	var usernames []string
	for {
		u := new(utmpx)
		if err := binary.Read(f, binary.LittleEndian, u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("Unable to read: %w", err)
		}
		if u.Type == typeUser {
			end := bytes.IndexByte(u.Name[:], 0)
			usernames = append(usernames, string(u.Name[:end]))
		}
	}

	return usernames, nil
}

//GetUsers returns the users currently signed in, or an error if one occurred
func GetUsers() ([]string, error) {
	for _, path := range UtmpLocations {
		usernames, err := readUtmp(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Println("WARN: Error reading utmp file:", err)
			continue
		}
		return coalesce(usernames), nil
	}
	for _, path := range UtmpxLocations {
		usernames, err := readUtmpx(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Println("WARN: Error reading utmpx file:", err)
			continue
		}
		return coalesce(usernames), nil
	}
	return nil, errors.New("Unable to find utmp/utmpx file")
}
