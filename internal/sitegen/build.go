package sitegen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ContentDir   string
	TemplatesDir string
	StaticDir    string
	OutputDir    string
}

func Build(config Config) error {
	config, err := normalizeConfig(config)
	if err != nil {
		return err
	}
	if err := validateInputs(config); err != nil {
		return err
	}

	posts, err := LoadPosts(filepath.Join(config.ContentDir, "posts"))
	if err != nil {
		return err
	}

	outputParent := filepath.Dir(config.OutputDir)
	if err := os.MkdirAll(outputParent, 0o755); err != nil {
		return fmt.Errorf("create output parent %q: %w", outputParent, err)
	}
	temporary, err := os.MkdirTemp(outputParent, ".sitegen-")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	defer os.RemoveAll(temporary)

	if err := renderSite(config.TemplatesDir, temporary, posts); err != nil {
		return err
	}
	if err := CopyStatic(config.StaticDir, temporary); err != nil {
		return err
	}
	if err := validateOutput(temporary); err != nil {
		return err
	}
	if err := replaceOutput(temporary, config.OutputDir); err != nil {
		return err
	}

	return nil
}

func normalizeConfig(config Config) (Config, error) {
	fields := []struct {
		name  string
		value *string
	}{
		{"content", &config.ContentDir},
		{"templates", &config.TemplatesDir},
		{"static", &config.StaticDir},
		{"output", &config.OutputDir},
	}
	for _, field := range fields {
		if *field.value == "" {
			return Config{}, fmt.Errorf("%s directory is required", field.name)
		}
		absolute, err := filepath.Abs(*field.value)
		if err != nil {
			return Config{}, fmt.Errorf("resolve %s directory %q: %w", field.name, *field.value, err)
		}
		*field.value = filepath.Clean(absolute)
	}
	return config, nil
}

func validateInputs(config Config) error {
	for name, path := range map[string]string{
		"content":   config.ContentDir,
		"templates": config.TemplatesDir,
		"static":    config.StaticDir,
	} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("validate %s directory %q: %w", name, path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("validate %s directory %q: not a directory", name, path)
		}
	}
	return nil
}

func replaceOutput(source, destination string) error {
	backup := destination + ".previous"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove stale output backup %q: %w", backup, err)
	}

	destinationExists := true
	if err := os.Rename(destination, backup); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("back up existing output %q: %w", destination, err)
		}
		destinationExists = false
	}

	if err := os.Rename(source, destination); err != nil {
		if destinationExists {
			_ = os.Rename(backup, destination)
		}
		return fmt.Errorf("publish output %q: %w", destination, err)
	}
	if destinationExists {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove output backup %q: %w", backup, err)
		}
	}
	return nil
}

func validateOutput(output string) error {
	for _, name := range []string{"index.html", "404.html"} {
		info, err := os.Stat(filepath.Join(output, name))
		if err != nil {
			return fmt.Errorf("validate generated file %q: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("validate generated file %q: not a regular file", name)
		}
	}
	return nil
}
