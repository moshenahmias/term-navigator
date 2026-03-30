package main

import (
	"context"
	"errors"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/moshenahmias/term-navigator/internal/ui"
)

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		log.Fatal(err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

func run(ctx context.Context) error {
	cfg, err := LoadConfig()

	if err != nil || len(cfg.Devices) == 0 {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}

		cfg = &defaultConfig
	}

	devs := make(map[string]file.Explorer, len(cfg.Devices))

	for _, devCfg := range cfg.Devices {
		constructor, ok := factory[devCfg.Type]
		if !ok {
			return errors.New("unknown device type: " + devCfg.Type)
		}
		dev, err := constructor(ctx, &devCfg)
		if err != nil {
			return errors.New("failed to create device: " + err.Error())
		}

		if _, exists := devs[devCfg.Name]; exists {
			return errors.New("duplicate device name: " + devCfg.Name)
		}

		devs[devCfg.Name] = dev
	}

	if len(devs) == 0 {
		return errors.New("no valid devices found in config")
	}

	app, err := ui.NewApp(ctx, devs, cfg.Left, cfg.Right, 120, 30)

	if err != nil {
		return errors.New("failed to create app: " + err.Error())
	}

	p := tea.NewProgram(app)

	if _, err := p.Run(); err != nil {
		return errors.New("failed to run program: " + err.Error())
	}

	return nil
}
