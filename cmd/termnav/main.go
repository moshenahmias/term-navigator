package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/moshenahmias/term-navigator/internal/ui"
)

var Version = "dev"
var defaultConfigPath string
var validDevName = regexp.MustCompile(`^[A-Za-z_-]+$`)

var (
	configPathFlag *string
	versionFlag    = flag.Bool("version", false, "Print version and exit")
)

func init() {
	defaultConfigPath, _ = config.Path()
	configPathFlag = flag.String("config", defaultConfigPath, "Path to config file")
}

func isValidDevName(s string) bool {
	return validDevName.MatchString(s)
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

	var errs []error

	cfg, err := config.Load(*configPathFlag)

	if err != nil && (!errors.Is(err, os.ErrNotExist) || *configPathFlag != defaultConfigPath) {
		errs = append(errs, err)
	}

	devs := make(map[string]file.Explorer, len(cfg.Devices))

	for i, devCfg := range cfg.Devices {
		if !isValidDevName(devCfg.Name) {
			return fmt.Errorf("device %d name is invalid (allowed: A-Z, a-z, _ or -)", i)
		}

		if devCfg.Type == "" {
			return fmt.Errorf("device %d (%s) missing type", i, devCfg.Name)
		}

		constructor, ok := factory[devCfg.Type]
		if !ok {
			return errors.New("unknown device type: " + devCfg.Type)
		}
		constructed, err := constructor(ctx, &devCfg)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create device %d (%s): %w", i, devCfg.Name, err))
		}

		if len(constructed) > 0 {
			for name, dev := range constructed {
				if _, exists := devs[name]; exists {
					return fmt.Errorf("duplicate device name: %s", name)
				}

				devs[name] = dev
			}
		}
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

	if len(errs) > 0 {
		go func() {
			defer close(done)
			app.Send(ui.NewLongErrorMsgFromErrors(errs...))
		}()
	}

	_, err = p.Run()

	if len(errs) > 0 {
		<-done
	}

	return err
}
