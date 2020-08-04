package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/korylprince/printer-manager-cups/cups"
)

const apiPath = "/users/%s/printers"

func GetPrinters(apiBase string, usernames []string) ([]*cups.Printer, error) {
	printerSet := make(map[string]*cups.Printer)

	for _, username := range usernames {
		resp, err := http.Get(apiBase + fmt.Sprintf(apiPath, username))
		if err != nil {
			return nil, fmt.Errorf("Unable to query printers: %v", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			// skip unknown users
			resp.Body.Close()
			continue
		}

		printers := make([]*cups.Printer, 0)
		d := json.NewDecoder(resp.Body)
		if err := d.Decode(&printers); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("Unable to decode response: %v", err)
		}

		resp.Body.Close()

		for _, p := range printers {
			printerSet[p.ID] = p
		}
	}

	// coalesce printers
	printers := make([]*cups.Printer, 0, len(printerSet))
	for _, p := range printerSet {
		printers = append(printers, p)
	}

	return printers, nil
}
