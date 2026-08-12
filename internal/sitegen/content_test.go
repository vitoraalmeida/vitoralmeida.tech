package sitegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPostsOrdersAndBuildsSlug(t *testing.T) {
	posts, err := LoadPosts(filepath.Join("testdata", "site", "content", "posts"))
	if err != nil {
		t.Fatalf("LoadPosts() error = %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("LoadPosts() returned %d posts, want 2", len(posts))
	}
	if posts[0].Slug != "second-post" || posts[1].Slug != "hello" {
		t.Fatalf("LoadPosts() slugs = [%q, %q], want [second-post, hello]", posts[0].Slug, posts[1].Slug)
	}
	if posts[1].DateISO != "2026-08-01" {
		t.Fatalf("LoadPosts() DateISO = %q, want 2026-08-01", posts[1].DateISO)
	}
	content := string(posts[1].Content)
	for _, fragment := range []string{
		`<figure class="article-figure">`,
		`<picture><source type="image/webp" srcset="/public/posts/hello/diagram-480w.webp 480w, /public/posts/hello/diagram-800w.webp 800w" sizes="(max-width: 700px) 100vw, 70ch" /><img src="/public/posts/hello/diagram.png" alt="Diagram" fetchpriority="high"/></picture>`,
		`<img src="/public/posts/hello/hero.png" alt="Hero" loading="lazy"/>`,
		`<figcaption class="article-figure__caption">Diagram</figcaption>`,
	} {
		if !strings.Contains(content, fragment) {
			t.Errorf("LoadPosts() content missing %q:\n%s", fragment, content)
		}
	}
}

func TestRenderMarkdownBuildsNestedTableOfContents(t *testing.T) {
	content, tableOfContents := renderMarkdown([]byte("## First heading\n\n### Detail\n\n## First heading\n"), "")
	for _, fragment := range []string{
		`<h2 id="first-heading">First heading<a class="heading-permalink" href="#first-heading" aria-label="Link permanente para First heading">#</a></h2>`,
		`<h3 id="detail">Detail<a class="heading-permalink" href="#detail" aria-label="Link permanente para Detail">#</a></h3>`,
		`<h2 id="first-heading-1">First heading<a class="heading-permalink" href="#first-heading-1" aria-label="Link permanente para First heading">#</a></h2>`,
	} {
		if !strings.Contains(string(content), fragment) {
			t.Errorf("renderMarkdown() content missing %q:\n%s", fragment, content)
		}
	}
	if len(tableOfContents) != 2 {
		t.Fatalf("renderMarkdown() TOC has %d top-level items, want 2", len(tableOfContents))
	}
	if tableOfContents[0].ID != "first-heading" || tableOfContents[0].Title != "First heading" {
		t.Errorf("renderMarkdown() first TOC item = %#v", tableOfContents[0])
	}
	if len(tableOfContents[0].Children) != 1 || tableOfContents[0].Children[0].ID != "detail" {
		t.Errorf("renderMarkdown() nested TOC items = %#v", tableOfContents[0].Children)
	}
	if tableOfContents[1].ID != "first-heading-1" {
		t.Errorf("renderMarkdown() duplicate heading ID = %q, want first-heading-1", tableOfContents[1].ID)
	}
}

func TestRenderMarkdownOmitsShortTableOfContents(t *testing.T) {
	_, tableOfContents := renderMarkdown([]byte("## First heading\n\n## Second heading\n"), "")
	if tableOfContents != nil {
		t.Fatalf("renderMarkdown() TOC = %#v, want nil", tableOfContents)
	}
}

func TestLoadPostsReportsMissingFiles(t *testing.T) {
	postsRoot := t.TempDir()
	postDir := filepath.Join(postsRoot, "01-missing")
	if err := os.Mkdir(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "meta.toml"), []byte("title = \"Missing\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPosts(postsRoot)
	if err == nil || !strings.Contains(err.Error(), `load post "01-missing" content`) {
		t.Fatalf("LoadPosts() error = %v, want contextual missing-content error", err)
	}
}

func TestLoadPostsReportsInvalidMetadata(t *testing.T) {
	postsRoot := t.TempDir()
	postDir := filepath.Join(postsRoot, "01-invalid")
	if err := os.Mkdir(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "meta.toml"), []byte("not valid toml ="), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(postDir, "post.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPosts(postsRoot)
	if err == nil || !strings.Contains(err.Error(), `load post "01-invalid" metadata`) {
		t.Fatalf("LoadPosts() error = %v, want contextual metadata error", err)
	}
}
