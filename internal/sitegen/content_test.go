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
