//go:build !gui
// +build !gui

package main

import (
	"log"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := DefaultConfig()
	log.Printf("starting auto rename service with config: watchRoot=%s eventDelay=%s stableChecks=%d stableInterval=%s",
		cfg.WatchRoot, cfg.EventDelay, cfg.FileStableChecks, cfg.FileStableInterval)

	service, err := NewRenamerService(cfg)
	if err != nil {
		log.Fatalf("service init failed: %v", err)
	}
	defer func() {
		if err := service.Close(); err != nil {
			log.Printf("close watcher error: %v", err)
		}
	}()

	RunWithSignal(service)
}
