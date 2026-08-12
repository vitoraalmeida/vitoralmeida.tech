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

// run despacha o primeiro argumento para o comando correspondente (build,
// check ou version) e reporta erro para comandos desconhecidos ou ausentes.
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("expected a command: build, check, or version")
	}

	switch args[0] {
	case "build":
		return runConfiguredCommand("build", args[1:], sitegen.Build)
	case "check":
		return runConfiguredCommand("check", args[1:], sitegen.Check)
	case "version":
		fmt.Println(version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// runConfiguredCommand define as flags do CLI, aplica as configurações na
// Config compartilhada e executa a função (Build ou Check) com esses valores.
func runConfiguredCommand(command string, args []string, execute func(sitegen.Config) error) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	config := sitegen.Config{}
	flags.StringVar(&config.ContentDir, "content", "./content", "content directory")
	flags.StringVar(&config.TemplatesDir, "templates", "./templates", "templates directory")
	flags.StringVar(&config.StaticDir, "static", "./static", "static files directory")
	flags.StringVar(&config.OutputDir, "output", "./dist", "output directory")
	flags.BoolVar(&config.NoIndex, "noindex", false, "add robots noindex meta to every page")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	return execute(config)
}
