package cups

import (
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"os/user"
	"time"

	"github.com/phin1x/go-ipp"
)

const DefaultCacheTimeout = time.Minute * 5

const EverywhereDriver = "everywhere"

// Client is a CUPS client that connects over unix sockets
type Client struct {
	client       *ipp.IPPClient
	adapter      ipp.Adapter
	CacheTimeout time.Duration
	cache        map[string]string
	cacheTime    time.Time
}

// New returns a new client or an error if one occurred
func New() (*Client, error) {
	// set user field
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Unable to lookup current user: %w", err)
	}
	adapter := ipp.NewSocketAdapter("localhost:631", false)
	return &Client{
		client:       ipp.NewIPPClientWithAdapter(user.Username, adapter),
		adapter:      adapter,
		CacheTimeout: DefaultCacheTimeout,
	}, nil
}

func (c *Client) adminURL() string {
	return c.adapter.GetHttpUri("admin", "")
}

func (c *Client) getPPDs() (map[string]string, error) {
	r := ipp.NewRequest(ipp.OperationCupsGetPPDs, rand.Int31())
	r.OperationAttributes[ipp.AttributeRequestedAttributes] = []string{ipp.AttributePPDMakeAndModel, ipp.AttributePPDName}
	resp, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to complete IPP request: %w", err)
	}

	ppds := make(map[string]string)

	for _, a := range resp.PrinterAttributes {
		if val := a[ipp.AttributePPDMakeAndModel]; len(val) == 1 {
			if val2 := a[ipp.AttributePPDName]; len(val2) == 1 {
				ppds[(val[0].Value).(string)] = (val2[0].Value).(string)
			}
		}
	}

	return ppds, nil
}

// GetPPDs returns the a mapping of make-and-model to name for all PPDs installed or an error if one occurred
func (c *Client) GetPPDs() (map[string]string, error) {
	if c.cache != nil && time.Since(c.cacheTime) < c.CacheTimeout {
		return c.cache, nil
	}

	ppds, err := c.getPPDs()
	if err != nil {
		return nil, err
	}

	c.cache = ppds
	c.cacheTime = time.Now()
	return ppds, nil
}

// GetDefault returns the id of the default Printer or an error if one occurred
func (c *Client) GetDefault() (string, error) {
	r := ipp.NewRequest(ipp.OperationCupsGetDefault, rand.Int31())
	r.OperationAttributes[ipp.AttributeRequestedAttributes] = []string{ipp.AttributePrinterName}
	resp, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return "", fmt.Errorf("Unable to complete IPP request: %w", err)
	}

	if len(resp.PrinterAttributes) != 1 {
		return "", errors.New("Server did not return a default printer")
	}

	return (resp.PrinterAttributes[0][ipp.AttributePrinterName][0].Value).(string), nil
}

// CUPS hold CUPS-specific driver options
type CUPS struct {
	DriverName      []string          `json:"driver_name"`
	URITemplate     string            `json:"uri_template"`
	DefaultPriority int               `json:"default_priority"`
	Options         map[string]string `json:"options"`
	Override        struct {
		Name     string `json:"name"`
		Location string `json:"location"`
	} `json:"override"`
}

// Driver hold driver options
type Driver struct {
	*CUPS `json:"cups"`
}

// Printer represents a CUPs printer
type Printer struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Name     string `json:"name"`
	Location string `json:"location"`
	*Driver  `json:"driver"`
}

func (p *Printer) GetName() string {
	if p.Driver != nil && p.Driver.CUPS != nil && p.Driver.CUPS.Override.Name != "" {
		return p.Driver.CUPS.Override.Name
	}
	return p.Name
}

func (p *Printer) GetLocation() string {
	if p.Driver != nil && p.Driver.CUPS != nil && p.Driver.CUPS.Override.Location != "" {
		return p.Driver.CUPS.Override.Location
	}
	return p.Location
}

// GetPrinters returns all the installed Printers or an error if one occurred
func (c *Client) GetPrinters() ([]*Printer, error) {
	r := ipp.NewRequest(ipp.OperationCupsGetPrinters, rand.Int31())
	r.OperationAttributes[ipp.AttributeRequestedAttributes] = []string{ipp.AttributePrinterName, ipp.AttributeDeviceURI, ipp.AttributePrinterInfo, ipp.AttributePrinterLocation}
	resp, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to complete IPP request: %w", err)
	}

	printers := make([]*Printer, 0, len(resp.PrinterAttributes))

	for _, a := range resp.PrinterAttributes {
		p := new(Printer)

		if val := a[ipp.AttributePrinterName]; len(val) == 1 {
			p.ID = (val[0].Value).(string)
		}

		// we're returning full URI here instead of trying to parse out hostname
		if val := a[ipp.AttributeDeviceURI]; len(val) == 1 {
			p.Hostname = (val[0].Value).(string)
		}

		if val := a[ipp.AttributePrinterInfo]; len(val) == 1 {
			p.Name = (val[0].Value).(string)
		}

		if val := a[ipp.AttributePrinterLocation]; len(val) == 1 {
			p.Location = (val[0].Value).(string)
		}

		printers = append(printers, p)
	}

	return printers, nil
}

// AddOrModify creates or updates the Printer or returns an error if one occurred
func (c *Client) AddOrModify(p *Printer) error {
	// skip misconfigured drivers
	if p.Driver == nil || p.Driver.CUPS == nil {
		return errors.New("Missing driver configuration")
	}
	//find first matching PPD
	ppds, err := c.GetPPDs()
	if err != nil {
		return fmt.Errorf("Unable to get PPDs: %w", err)
	}

	ppd := ""
	var everywhere bool
	for _, p := range p.DriverName {
		if name, ok := ppds[p]; ok {
			ppd = name
			break
		} else if p == EverywhereDriver {
			everywhere = true
		}
	}

	if ppd == "" {
		// only create IPP Everywhere printer if no PPDs are found
		if everywhere {
			if err := c.CreateIPPEverywhere(p); err != nil {
				return fmt.Errorf("could not create IPP Everywhere printer: %w", err)
			}
		} else {
			return errors.New("No matching PPDs found")
		}
	}

	r := ipp.NewRequest(ipp.OperationCupsAddModifyPrinter, rand.Int31())
	r.OperationAttributes[ipp.AttributePrinterURI] = c.adapter.GetHttpUri("printers", p.ID)
	r.OperationAttributes[ipp.AttributeDeviceURI] = fmt.Sprintf(p.URITemplate, p.Hostname)
	if ppd != "" {
		r.OperationAttributes[ipp.AttributePPDName] = ppd
	}
	r.OperationAttributes[ipp.AttributePrinterInfo] = p.GetName()
	r.OperationAttributes[ipp.AttributePrinterLocation] = p.GetLocation()
	r.OperationAttributes[ipp.AttributePrinterIsAcceptingJobs] = true
	r.OperationAttributes[ipp.AttributePrinterState] = ipp.PrinterStateIdle
	// FIXME: update once https://github.com/phin1x/go-ipp/pull/37 is merged
	ipp.AttributeTagMapping["printer-is-temporary"] = ipp.TagBoolean
	r.OperationAttributes["printer-is-temporary"] = false
	if _, err := c.client.SendRequest(c.adminURL(), r, nil); err != nil {
		return fmt.Errorf("Unable to add or modify printer: %w", err)
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
		return fmt.Errorf("Unable to set printer options: %w", err)
	}

	return nil
}

// CreateIPPEverywhere creates the Printer as an IPP Everywhere printer or returns an error if one occurred
func (c *Client) CreateIPPEverywhere(p *Printer) error {
	// https://github.com/apple/cups/issues/5919
	// first create local printer, then update to make permanent
	r := ipp.NewRequest(ipp.OperationCupsCreateLocalPrinter, rand.Int31())
	r.OperationAttributes[ipp.AttributePrinterURI] = c.adapter.GetHttpUri("printers", p.ID)
	r.PrinterAttributes[ipp.AttributePrinterName] = p.ID
	r.PrinterAttributes[ipp.AttributeDeviceURI] = fmt.Sprintf(p.URITemplate, p.Hostname)
	r.PrinterAttributes[ipp.AttributePrinterInfo] = p.GetName()
	r.PrinterAttributes[ipp.AttributePrinterLocation] = p.GetLocation()
	r.OperationAttributes[ipp.AttributePrinterIsAcceptingJobs] = true
	r.OperationAttributes[ipp.AttributePrinterState] = ipp.PrinterStateIdle
	if _, err := c.client.SendRequest(c.adminURL(), r, nil); err != nil {
		ippErr := new(ipp.IPPError)
		if errors.As(err, ippErr) && ippErr.Status == ipp.StatusErrorNotPossible {
			// printer is already created,
			return nil
		}
		return fmt.Errorf("Unable to create local printer: %w", err)
	}

	return nil
}

// Delete deletes the Printer or returns an error if one occurred
func (c *Client) Delete(p *Printer) error {
	r := ipp.NewRequest(ipp.OperationCupsDeletePrinter, rand.Int31())
	r.OperationAttributes[ipp.AttributePrinterURI] = c.adapter.GetHttpUri("printers", p.ID)
	_, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return fmt.Errorf("Unable to complete IPP request: %w", err)
	}
	return nil
}

// SetDefault sets the Printer as default or returns an error if one occurred
func (c *Client) SetDefault(p *Printer) error {
	r := ipp.NewRequest(ipp.OperationCupsSetDefault, rand.Int31())
	r.OperationAttributes[ipp.AttributePrinterURI] = c.adapter.GetHttpUri("printers", p.ID)
	_, err := c.client.SendRequest(c.adminURL(), r, nil)
	if err != nil {
		return fmt.Errorf("Unable to complete IPP request: %w", err)
	}
	return nil
}

// ClearCache clears the clients PPD cache
func (c *Client) ClearCache() {
	c.cache = nil
}
