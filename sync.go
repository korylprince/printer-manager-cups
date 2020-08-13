package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/korylprince/printer-manager-cups/cache"
	"github.com/korylprince/printer-manager-cups/cups"
	"github.com/korylprince/printer-manager-cups/httpapi"
	"github.com/korylprince/printer-manager-cups/user"
)

func Sync(config *Config, usernames []string) error {
	log.Println("INFO: Starting sync")
	// get users
	allUsers, err := user.GetUsers()
	if err != nil {
		return fmt.Errorf("Unable to get users: %v", err)
	}

	// filter ignored users
	var users []string
outerIgnored:
	for _, u := range allUsers {
		for _, i := range config.IgnoreUsers {
			if u == i {
				continue outerIgnored
			}
		}
		users = append(users, u)
	}

	for _, u := range usernames {
		users = append(users, u)
	}

	log.Println("INFO: Getting printers for:", strings.Join(users, ", "))

	// get api printers
	printers, err := httpapi.GetPrinters(config.APIBase, users)
	if err != nil {
		return fmt.Errorf("Unable to get API printers: %v", err)
	}

	log.Println("INFO: Got", len(printers), "printers from API")

	// cache api printer ids
	pCache, err := cache.Read(config.CachePath)
	if err != nil {
		return fmt.Errorf("Unable to read cache: %v", err)
	}

	for _, p := range printers {
		pCache[p.ID] = time.Now().Add(config.CacheTime)
	}

	if err = pCache.Write(config.CachePath); err != nil {
		return fmt.Errorf("Unable to update cache: %v", err)
	}

	errPrinters := make(map[string]*cups.Printer)

	// sync api printers to cups
	for _, p := range printers {
		if err = p.AddOrModify(); err != nil {
			log.Printf("WARN: Unable to add or modify printer %s (%s): %v\n", p.ID, p.Hostname, err)
			errPrinters[p.ID] = p
			continue
		}
		log.Printf("INFO: Added/Modified printer: %s (%s)\n", p.ID, p.Hostname)
	}

	// get cups printers
	cupsPrinters, err := cups.GetPrinters()
	if err != nil {
		if !strings.Contains(err.Error(), "No destinations added.") {
			return fmt.Errorf("Unable to get CUPS printers: %v", err)
		}
	}

	log.Println("INFO: Got", len(cupsPrinters), "printers from CUPS")

	// remove matching, unmanaged printers
	for _, cp := range cupsPrinters {
		for _, p := range printers {
			// skip if same printer
			if cp.ID == p.ID {
				continue
			}
			// skip error printers
			if _, ok := errPrinters[p.ID]; ok {
				continue
			}
			// skip if doesn't match
			if !strings.Contains(cp.Hostname, p.Hostname) {
				continue
			}

			if err = cp.Delete(); err != nil {
				log.Printf("WARN: Unable to remove matched printer %s: %v\n", cp.ID, err)
				continue
			}
			log.Printf("INFO: Removed matching printer %s (%s): matched %s (%s)\n", cp.ID, cp.Hostname, p.ID, p.Hostname)
		}
	}

	// delete expired printers
	var deleted []string
outerExpired:
	for id, expiration := range pCache {
		if !expiration.Before(time.Now()) {
			continue
		}

		for _, cp := range cupsPrinters {
			if id == cp.ID {
				if err = cp.Delete(); err != nil {
					log.Printf("WARN: Unable to delete expired printer %s (%s): %v\n", cp.ID, cp.Hostname, err)
					continue outerExpired
				}
				log.Printf("INFO: Deleted expired printer %s (%s)\n", cp.ID, cp.Hostname)
				break
			}
		}
		// printer deleted successfully or not found
		deleted = append(deleted, id)
	}

	// get default printer
	cupsDefault, err := cups.GetDefault()
	if err != nil {
		log.Println("WARN: Unable to get default printer:", err)
	}

	// elect default printer
	var def *cups.Printer
	for _, p := range printers {
		if p.ID == cupsDefault {
			def = p
			break
		}
	}

	for _, p := range printers {
		// skip error printers
		if _, ok := errPrinters[p.ID]; ok {
			continue
		}

		if def == nil || p.DefaultPriority > def.DefaultPriority {
			def = p
		}
	}

	// set default printer
	if def != nil && def.ID != cupsDefault {
		if err = def.SetDefault(); err != nil {
			log.Printf("WARN: Unable to set default printer to %s (%s): %v\n", def.ID, def.Hostname, err)
		} else {
			log.Printf("INFO: Set default printer to %s (%s)\n", def.ID, def.Hostname)
		}
	}

	// purge expired printers from cache
	if err = cache.Purge(config.CachePath, deleted); err != nil {
		log.Println("WARN: Unable to purge cache:", err)
	}

	log.Println("INFO: Sync completed successfully")
	return nil
}

func ClearCache(config *Config) error {
	log.Println("INFO: Clearing cached printers")
	// cache api printer ids
	pCache, err := cache.Read(config.CachePath)
	if err != nil {
		return fmt.Errorf("Unable to read cache: %v", err)
	}

	// get cups printers
	cupsPrinters, err := cups.GetPrinters()
	if err != nil {
		if !strings.Contains(err.Error(), "No destinations added.") {
			return fmt.Errorf("Unable to get CUPS printers: %v", err)
		}
	}

	log.Println("INFO: Got", len(cupsPrinters), "printers from CUPS")

	// delete expired printers
	var deleted []string
outerExpired:
	for id := range pCache {
		for _, cp := range cupsPrinters {
			if id == cp.ID {
				if err = cp.Delete(); err != nil {
					log.Printf("WARN: Unable to delete expired printer %s (%s): %v\n", cp.ID, cp.Hostname, err)
					continue outerExpired
				}
				log.Printf("INFO: Deleted expired printer %s (%s)\n", cp.ID, cp.Hostname)
				break
			}
		}
		// printer deleted successfully or not found
		deleted = append(deleted, id)
	}

	// purge expired printers from cache
	if err = cache.Purge(config.CachePath, deleted); err != nil {
		log.Println("WARN: Unable to purge cache:", err)
	}

	log.Println("INFO: Cache cleared successfully")
	return nil
}
