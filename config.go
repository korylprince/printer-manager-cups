package main

import "time"

type Config struct {
	APIBase      string        `required:"true"`
	CachePath    string        `default:"/etc/printer-manager"`
	CacheTime    time.Duration `default:"336h"` // 14 days
	SyncInterval time.Duration `default:"1h"`
	IgnoreUsers  []string      `default:"root"`
}
