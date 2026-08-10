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
		"styles/global.css", "public/posts/hello/diagram.png", "public/posts/hello/hero.png",
		"og-image.png", "robots.txt", "sitemap.xml", "feed.xml",
	}
	for _, name := range required {
		if info, err := os.Stat(filepath.Join(output, filepath.FromSlash(name))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("generated file %q missing or invalid: %v", name, err)
		}
	}

	for _, name := range []string{"index.html", "blog/hello.html"} {
		assertGolden(t, name, readTestFile(t, filepath.Join(output, filepath.FromSlash(name))))
	}

	index := readTestFile(t, filepath.Join(output, "index.html"))
	for _, want := range []string{
		`<link rel="canonical" href="https://vitoralmeida.tech/" />`,
		`"@type":"WebSite"`,
		`<meta property="og:type" content="website" />`,
		`<meta property="og:image" content="https://vitoralmeida.tech/og-image.png" />`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	post := readTestFile(t, filepath.Join(output, "blog", "hello.html"))
	for _, want := range []string{
		`<link rel="canonical" href="https://vitoralmeida.tech/blog/hello" />`,
		`"@type":"BlogPosting"`,
		`"datePublished":"2026-08-01"`,
		`<meta property="og:type" content="article" />`,
		`<meta property="article:published_time" content="2026-08-01" />`,
	} {
		if !strings.Contains(post, want) {
			t.Errorf("blog/hello.html missing %q", want)
		}
	}
	sitemap := readTestFile(t, filepath.Join(output, "sitemap.xml"))
	for _, want := range []string{
		"<loc>https://vitoralmeida.tech/</loc>",
		"<loc>https://vitoralmeida.tech/blog</loc>",
		"<loc>https://vitoralmeida.tech/blog/hello</loc>",
		"<loc>https://vitoralmeida.tech/blog/second-post</loc>",
		"<loc>https://vitoralmeida.tech/feed.xml</loc>",
		"<lastmod>2026-08-01</lastmod>",
		"<lastmod>2026-08-02</lastmod>",
	} {
		if !strings.Contains(sitemap, want) {
			t.Errorf("sitemap.xml missing %q", want)
		}
	}
	feed := readTestFile(t, filepath.Join(output, "feed.xml"))
	for _, want := range []string{
		`<rss version="2.0"`,
		`<language>pt-br</language>`,
		"<link>https://vitoralmeida.tech/blog/hello</link>",
		`<title>Hello &lt;world&gt;</title>`,
		"<guid isPermaLink=\"true\">https://vitoralmeida.tech/blog/second-post</guid>",
		"<pubDate>Sat, 01 Aug 2026 00:00:00 +0000</pubDate>",
		"<content:encoded>",
		"<author>vitor@vitoralmeida.tech (Vitor Almeida)</author>",
	} {
		if !strings.Contains(feed, want) {
			t.Errorf("feed.xml missing %q", want)
		}
	}

	newestPost := readTestFile(t, filepath.Join(output, "blog", "second-post.html"))
	if !strings.Contains(newestPost, `href="/blog/hello">Older: Hello &lt;world&gt;</a>`) {
		t.Errorf("newest post missing older-post navigation: %s", newestPost)
	}
	oldestPost := readTestFile(t, filepath.Join(output, "blog", "hello.html"))
	if !strings.Contains(oldestPost, `href="/blog/second-post">Newer: Second post</a>`) {
		t.Errorf("oldest post missing newer-post navigation: %s", oldestPost)
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
