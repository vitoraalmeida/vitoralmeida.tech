package sitegen

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

func TestCheckDoesNotWriteOutput(t *testing.T) {
	config := fixtureConfig(filepath.Join(t.TempDir(), "dist"))
	if err := Check(config); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if _, err := os.Stat(config.OutputDir); !os.IsNotExist(err) {
		t.Fatalf("Check() created output %q", config.OutputDir)
	}
}

func TestBuildProducesExpectedSite(t *testing.T) {
	output := filepath.Join(t.TempDir(), "dist")
	if err := Build(fixtureConfig(output)); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	required := []string{
		"index.html", "404.html", "blog.html", "blog/hello.html", "blog/second-post.html",
		"styles/global.css", "public/posts/hello/diagram.png", "robots.txt",
	}
	for _, name := range required {
		if info, err := os.Stat(filepath.Join(output, filepath.FromSlash(name))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("generated file %q missing or invalid: %v", name, err)
		}
	}

	for _, name := range []string{"index.html", "blog/hello.html"} {
		assertGolden(t, name, readTestFile(t, filepath.Join(output, filepath.FromSlash(name))))
	}
}

func TestBuildFailurePreservesExistingOutput(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "dist")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(output, "sentinel.txt")
	if err := writeTestFile(sentinel, "original"); err != nil {
		t.Fatal(err)
	}

	templates := filepath.Join(root, "templates")
	if err := copyDirectoryContents(filepath.Join("testdata", "site", "templates"), templates); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(filepath.Join(templates, "base-template.gohtml"), `{{.UnknownField}}`); err != nil {
		t.Fatal(err)
	}
	config := fixtureConfig(output)
	config.TemplatesDir = templates
	if err := Build(config); err == nil {
		t.Fatal("Build() error = nil, want template execution error")
	}
	if got := readTestFile(t, sentinel); got != "original" {
		t.Fatalf("existing output changed after failed build: %q", got)
	}
}

func TestCheckRejectsOverlappingOutput(t *testing.T) {
	config := fixtureConfig(filepath.Join("testdata", "site", "static", "dist"))
	err := Check(config)
	if err == nil || !strings.Contains(err.Error(), "must not overlap") {
		t.Fatalf("Check() error = %v, want overlap error", err)
	}
}

func fixtureConfig(output string) Config {
	return Config{
		ContentDir: filepath.Join("testdata", "site", "content"), TemplatesDir: filepath.Join("testdata", "site", "templates"),
		StaticDir: filepath.Join("testdata", "site", "static"), OutputDir: output,
	}
}

func assertGolden(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("testdata", "expected", filepath.FromSlash(name))
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expected := readTestFile(t, path)
	if actual != expected {
		t.Fatalf("generated %s differs from golden file; run go test ./... -update to approve changes", name)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
