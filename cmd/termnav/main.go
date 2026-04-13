package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"

	tea "charm.land/bubbletea/v2"
	"github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/tncore"
)

var Version = "dev"
var defaultConfigPath string
var validDevName = regexp.MustCompile(`^[A-Za-z_-]+$`)

var (
	configPathFlag   *string
	versionFlag      bool
	loadDisabledFlag bool
)

func init() {
	defaultConfigPath, _ = config.Path()
	configPathFlag = flag.String("config", defaultConfigPath, "Path to config file")
	flag.BoolVar(&versionFlag, "v", false, "Print version and exit")
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.BoolVar(&loadDisabledFlag, "ld", false, "Print version and exit")
	flag.BoolVar(&loadDisabledFlag, "load-disabled", false, "Load disabled devices")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options]\n\n", os.Args[0])

		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  --config <path>        Path to config file (default: %s)\n", defaultConfigPath)
		fmt.Fprintf(os.Stderr, "  -v, --version          Print version and exit\n")
		fmt.Fprintf(os.Stderr, "  -ld, --load-disabled   Load disabled devices\n")
	}
}

func isValidDevName(s string) bool {
	return validDevName.MatchString(s)
}

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run(ctx context.Context) error {
	flag.Parse()

	if versionFlag {
		fmt.Println(Version)
		return nil
	}

	cfg, err := config.Load(*configPathFlag)

	if err != nil && (!errors.Is(err, os.ErrNotExist) || *configPathFlag != defaultConfigPath) {
		fmt.Fprintln(os.Stderr, err)
	}

	for i, devCfg := range cfg.Devices {
		if !isValidDevName(devCfg.Name) {
			return fmt.Errorf("device %d name is invalid (allowed: A-Z, a-z, _ or -)", i)
		}
	}

	constructors, err := buildConstructors(cfg)
	if err != nil {
		return err
	}

	if len(constructors) == 0 {
		return errors.New("no valid devices found in config")
	}

	app, err := tncore.NewApp(ctx, constructors, cfg.Left, cfg.Right, 120, 30)

	if err != nil {
		return errors.New("failed to create app: " + err.Error())
	}

	p := tea.NewProgram(app)
	app.Send = func(m tea.Msg) {
		p.Send(m)
	}

	_, err = p.Run()

	return err
}
