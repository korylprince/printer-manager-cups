package cups

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"

	"github.com/phin1x/go-ipp"
)

var ServerURL = "http://localhost:631/admin"

const requestRetryLimit = 3

var SocketSearchPaths = []string{"/var/run/cupsd", "/var/run/cups/cups.sock", "/run/cups/cups.sock"}
var CertSearchPaths = []string{"/etc/cups/certs/0", "/run/cups/certs/0"}

var socketNotFoundError = errors.New("Unable to locate CUPS socket")
var certNotFoundError = errors.New("Unable to locate CUPS certificate")

//GetSocket returns the path to the cupsd socket by searching SocketSearchPaths
func GetSocket() (string, error) {
	for _, path := range SocketSearchPaths {
		fi, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			} else if os.IsPermission(err) {
				return "", errors.New("Unable to access socket: Access denied")
			}
			return "", fmt.Errorf("Unable to access socket: %v", err)
		}

		if fi.Mode()&os.ModeSocket != 0 {
			return path, nil
		}
	}

	return "", socketNotFoundError
}

//GetCert returns the current CUPs authentication certificate by searching CertSearchPaths
func GetCert() (string, error) {
	for _, path := range CertSearchPaths {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			} else if os.IsPermission(err) {
				return "", errors.New("Unable to access certificate: Access denied")
			}
			return "", fmt.Errorf("Unable to access certificate: %v", err)
		}
		defer f.Close()

		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, f); err != nil {
			return "", fmt.Errorf("Unable to access certificate: %v", err)
		}
		return buf.String(), nil
	}

	return "", certNotFoundError
}

//DoRequest performs the given IPP request to the given URL, returning the IPP response or an error if one occurred
func DoRequest(r *ipp.Request, url string) (*ipp.Response, error) {
	// set user field
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Unable to lookup current user: %v", err)
	}
	r.OperationAttributes["requesting-user-name"] = user.Username

	for i := 0; i < requestRetryLimit; i++ {
		// encode request
		payload, err := r.Encode()
		if err != nil {
			return nil, fmt.Errorf("Unable to encode IPP request: %v", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
		if err != nil {
			return nil, fmt.Errorf("Unable to create HTTP request: %v", err)
		}

		sock, err := GetSocket()
		if err != nil {
			return nil, err
		}

		// if cert isn't found, do a request to generate it
		cert, err := GetCert()
		if err != nil && err != certNotFoundError {
			return nil, err
		}

		req.Header.Set("Content-Length", strconv.Itoa(len(payload)))
		req.Header.Set("Content-Type", ipp.ContentTypeIPP)
		req.Header.Set("Authorization", fmt.Sprintf("Local %s", cert))

		unixClient := http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", sock)
				},
			},
		}

		// send request
		resp, err := unixClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Unable to perform HTTP request: %v", err)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			// retry with newly generated cert
			resp.Body.Close()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("Server did not return Status OK: %d", resp.StatusCode)
		}

		// buffer response to avoid read issues
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, resp.Body); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("Unable to buffer response: %v", err)
		}

		resp.Body.Close()

		// decode reply
		ippResp, err := ipp.NewResponseDecoder(bytes.NewReader(buf.Bytes())).Decode(nil)
		if err != nil {
			return nil, fmt.Errorf("Unable to decode IPP response: %v", err)
		}

		if err = ippResp.CheckForErrors(); err != nil {
			return nil, fmt.Errorf("Received error IPP response: %v", err)
		}

		return ippResp, nil
	}
	return nil, errors.New("Request retry limit exceeded")
}
