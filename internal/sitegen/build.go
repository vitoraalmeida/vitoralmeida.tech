package sitegen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ContentDir   string
	TemplatesDir string
	StaticDir    string
	OutputDir    string
}

// Build gera e valida o site fora do destino final para que uma falha não
// exponha uma saída incompleta nem substitua o último build válido.
func Build(config Config) error {
	config, err := normalizeConfig(config)
	if err != nil {
		return err
	}
	posts, err := check(config)
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
	if err := copyPostAssets(posts, temporary); err != nil {
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

// Check executa as mesmas validações de entrada do build sem gerar arquivos,
// permitindo detectar problemas de conteúdo rapidamente no desenvolvimento e CI.
func Check(config Config) error {
	config, err := normalizeConfig(config)
	if err != nil {
		return err
	}
	_, err = check(config)
	return err
}

// normalizeConfig transforma todos os caminhos em valores absolutos e limpos
// para que validações e comparações não dependam do diretório de trabalho nem
// de representações diferentes do mesmo caminho.
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

// validateInputs confirma que as fontes são diretórios utilizáveis e que não
// se sobrepõem à saída, evitando apagar fontes ou copiar arquivos gerados de
// volta para o próprio build.
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
	for name, input := range map[string]string{
		"content": config.ContentDir, "templates": config.TemplatesDir, "static": config.StaticDir,
	} {
		if pathContains(input, config.OutputDir) || pathContains(config.OutputDir, input) {
			return fmt.Errorf("output directory %q must not overlap %s directory %q", config.OutputDir, name, input)
		}
	}
	return nil
}

// pathContains determina se child é igual a parent ou está dentro dele usando
// caminhos relativos, centralizando a regra usada para detectar sobreposições.
func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

// replaceOutput publica o build pronto por renames e mantém temporariamente a
// saída anterior para poder restaurá-la caso a promoção da nova saída falhe.
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

// validateOutput exige os arquivos mínimos que caracterizam um site completo
// antes da publicação, impedindo que uma geração aparentemente bem-sucedida
// substitua a saída válida por um artefato essencialmente incompleto.
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
