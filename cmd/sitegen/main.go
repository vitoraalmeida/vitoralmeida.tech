package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/vitoraalmeida/vitoralmeida.tech/internal/sitegen"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("site generation failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("expected a command: build or version")
	}

	switch args[0] {
	case "build":
		return runBuild(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runBuild(args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	config := sitegen.Config{}
	flags.StringVar(&config.ContentDir, "content", "./content", "content directory")
	flags.StringVar(&config.TemplatesDir, "templates", "./templates", "templates directory")
	flags.StringVar(&config.StaticDir, "static", "./static", "static files directory")
	flags.StringVar(&config.OutputDir, "output", "./dist", "output directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	return sitegen.Build(config)
}
