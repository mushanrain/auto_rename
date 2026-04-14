//go:build gui
// +build gui

package main

import (
	"log"

	"auto_rename/gui"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := DefaultConfig()

	guiCfg := gui.Config{
		WatchRoot:          cfg.WatchRoot,
		EventDelay:         int(cfg.EventDelay.Seconds()),
		FileStableChecks:   cfg.FileStableChecks,
		FileStableInterval: int(cfg.FileStableInterval.Seconds()),
	}

	guiApp := gui.NewGuiApp(guiCfg)
	guiApp.Run()
}
