package cups

import (
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"os/user"

	"github.com/phin1x/go-ipp"
)

//Client is a CUPS client that connects over unix sockets
type Client struct {
	client  *ipp.IPPClient
	adapter ipp.Adapter
}

//New returns a new client or an error if one occurred
func New() (*Client, error) {
	// set user field
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Unable to lookup current user: %v", err)
	}
	adapter := ipp.NewSocketAdapter("localhost:631", false)
	return &Client{client: ipp.NewIPPClientWithAdapter(user.Username, adapter), adapter: adapter}, nil
}

func (c *Client) adminURL() string {
	return c.adapter.GetHttpUri("admin", "")
}

var cachedPPDs map[string]string = nil

//GetPPDs returns the a mapping of make-and-model to name for all PPDs installed or an error if one occurred
func (c *Client) GetPPDs() (map[string]string, error) {
	r := ipp.NewRequest(ipp.OperationCupsGetPPDs, rand.Int31())
	r.OperationAttributes["requested-attributes"] = []string{"ppd-make-and-model", "ppd-name"}
	resp, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return nil, err
	}

	ppds := make(map[string]string)

	for _, a := range resp.PrinterAttributes {
		if val := a["ppd-make-and-model"]; len(val) == 1 {
			if val2 := a["ppd-name"]; len(val2) == 1 {
				ppds[(val[0].Value).(string)] = (val2[0].Value).(string)
			}
		}
	}

	return ppds, err
}

func (c *Client) getCachedPPDs() (map[string]string, error) {
	if cachedPPDs != nil {
		return cachedPPDs, nil
	}

	ppds, err := c.GetPPDs()
	if err != nil {
		return nil, err
	}

	cachedPPDs = ppds
	return ppds, nil
}

//GetDefault returns the id of the default Printer or an error if one occurred
func (c *Client) GetDefault() (string, error) {
	r := ipp.NewRequest(ipp.OperationCupsGetDefault, rand.Int31())
	r.OperationAttributes["requested-attributes"] = []string{"printer-name"}
	resp, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return "", err
	}

	if len(resp.PrinterAttributes) != 1 {
		return "", errors.New("Server did not return a default printer")
	}

	return (resp.PrinterAttributes[0]["printer-name"][0].Value).(string), nil
}

//CUPS hold CUPS-specific driver options
type CUPS struct {
	DriverName      []string          `json:"driver_name"`
	URITemplate     string            `json:"uri_template"`
	DefaultPriority int               `json:"default_priority"`
	Options         map[string]string `json:"options"`
}

//Driver hold driver options
type Driver struct {
	*CUPS `json:"cups"`
}

//Printer represents a CUPs printer
type Printer struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Name     string `json:"name"`
	Location string `json:"location"`
	*Driver  `json:"driver"`
}

//GetPrinters returns all the installed Printers or an error if one occurred
func (c *Client) GetPrinters() ([]*Printer, error) {
	r := ipp.NewRequest(ipp.OperationCupsGetPrinters, rand.Int31())
	r.OperationAttributes["requested-attributes"] = []string{"printer-name", "device-uri", "printer-info", "printer-location"}
	resp, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return nil, err
	}

	printers := make([]*Printer, 0, len(resp.PrinterAttributes))

	for _, a := range resp.PrinterAttributes {
		p := new(Printer)

		if val := a["printer-name"]; len(val) == 1 {
			p.ID = (val[0].Value).(string)
		}

		// we're returning full URI here instead of trying to parse out hostname
		if val := a["device-uri"]; len(val) == 1 {
			p.Hostname = (val[0].Value).(string)
		}

		if val := a["printer-info"]; len(val) == 1 {
			p.Name = (val[0].Value).(string)
		}

		if val := a["printer-location"]; len(val) == 1 {
			p.Location = (val[0].Value).(string)
		}

		printers = append(printers, p)
	}

	return printers, err
}

//AddOrModify creates or updates the Printer or returns an error if one occurred
func (c *Client) AddOrModify(p *Printer) error {
	//find first matching PPD
	ppds, err := c.getCachedPPDs()
	if err != nil {
		return fmt.Errorf("Unable to get PPDs: %v", err)
	}

	ppd := ""
	for _, p := range p.DriverName {
		if name, ok := ppds[p]; ok {
			ppd = name
			break
		}
	}

	if ppd == "" {
		return errors.New("No matching PPDs found")
	}

	r := ipp.NewRequest(ipp.OperationCupsAddModifyPrinter, rand.Int31())
	r.OperationAttributes["printer-uri"] = c.adapter.GetHttpUri("printers", p.ID)
	r.OperationAttributes["device-uri"] = fmt.Sprintf(p.URITemplate, p.Hostname)
	r.OperationAttributes["ppd-name"] = ppd
	r.OperationAttributes["printer-info"] = p.Name
	r.OperationAttributes["printer-location"] = p.Location
	r.OperationAttributes["printer-is-accepting-jobs"] = true
	r.OperationAttributes["printer-state"] = ipp.PrinterStateIdle
	if _, err := c.client.SendRequest(c.adminURL(), r, nil); err != nil {
		return fmt.Errorf("Unable to add or modify printer: %v", err)
	}

	if len(p.Options) == 0 {
		return nil
	}

	//use lpadmin to update options
	options := []string{"-p", p.ID}
	for k, v := range p.Options {
		options = append(options, "-o", fmt.Sprintf("%s=%s", k, v))
	}
	cmd := exec.Command("lpadmin", options...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Unable to set printer options: %v", err)
	}

	return nil
}

//Delete deletes the Printer or returns an error if one occurred
func (c *Client) Delete(p *Printer) error {
	r := ipp.NewRequest(ipp.OperationCupsDeletePrinter, rand.Int31())
	r.OperationAttributes["printer-uri"] = c.adapter.GetHttpUri("printers", p.ID)
	_, err := c.client.SendRequest(c.adminURL(), r, nil)
	return err
}

//SetDefault sets the Printer as default or returns an error if one occurred
func (c *Client) SetDefault(p *Printer) error {
	r := ipp.NewRequest(ipp.OperationCupsSetDefault, rand.Int31())
	r.OperationAttributes["printer-uri"] = c.adapter.GetHttpUri("printers", p.ID)
	_, err := c.client.SendRequest(c.adminURL(), r, nil)
	return err
}
