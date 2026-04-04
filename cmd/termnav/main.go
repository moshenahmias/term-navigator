package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/moshenahmias/term-navigator/internal/ui"
)

var Version = "dev"

var (
	configPathFlag *string
	versionFlag    = flag.Bool("version", false, "Print version and exit")
)

func init() {
	path, _ := config.Path()
	configPathFlag = flag.String("config", path, "Path to config file")
}

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
	flag.Parse()

	if *versionFlag {
		fmt.Println(Version)
		return nil
	}

	cfg, cfgErr := config.Load(*configPathFlag)

	devs := make(map[string]file.Explorer, len(cfg.Devices))

	for i, devCfg := range cfg.Devices {

		if devCfg.Name == "" {
			return fmt.Errorf("device %d missing name", i)
		}
		if devCfg.Type == "" {
			return fmt.Errorf("device %d (%s) missing type", i, devCfg.Name)
		}

		constructor, ok := factory[devCfg.Type]
		if !ok {
			return errors.New("unknown device type: " + devCfg.Type)
		}
		dev, err := constructor(ctx, &devCfg)
		if err != nil {
			return fmt.Errorf("failed to create device %d (%s): %w", i, devCfg.Name, err)
		}

		if _, exists := devs[devCfg.Name]; exists {
			return fmt.Errorf("duplicate device name: %s", devCfg.Name)
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
	app.Send = func(m tea.Msg) {
		p.Send(m)
	}

	done := make(chan struct{})

	if cfgErr != nil {
		go func() {
			defer close(done)
			app.Send(ui.NewLongErrorMsg("failed reading config file: " + cfgErr.Error()))
		}()
	}

	_, err = p.Run()

	if cfgErr != nil {
		<-done
	}

	return err
}
