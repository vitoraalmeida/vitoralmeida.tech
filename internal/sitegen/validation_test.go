package sitegen

import (
	"html/template"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePosts(t *testing.T) {
	valid := Post{
		Title: "Title", Description: "Description", Date: "04/08/2026", Slug: "valid",
		SourceDir: filepath.Join("content", "posts", "01-valid"),
	}
	tests := []struct {
		name    string
		posts   []Post
		message string
	}{
		{"invalid directory", []Post{{Title: "Title", Description: "Description", Date: "04/08/2026", Slug: "Bad", SourceDir: "01-Bad"}}, "directory name"},
		{"missing title", []Post{{Description: "Description", Date: "04/08/2026", Slug: "valid", SourceDir: "01-valid"}}, `field "title" is required`},
		{"missing description", []Post{{Title: "Title", Date: "04/08/2026", Slug: "valid", SourceDir: "01-valid"}}, `field "description" is required`},
		{"invalid date", []Post{{Title: "Title", Description: "Description", Date: "2026-08-04", Slug: "valid", SourceDir: "01-valid"}}, "DD/MM/YYYY"},
		{"duplicate slug", []Post{valid, {Title: "Other", Description: "Other", Date: "05/08/2026", Slug: "valid", SourceDir: "02-valid"}}, "duplicates post"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePosts(test.posts)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("validatePosts() error = %v, want message containing %q", err, test.message)
			}
		})
	}
}

func TestValidateAssetReferencesReportsMissingFile(t *testing.T) {
	post := Post{
		Slug:      "hello",
		AssetsDir: t.TempDir(),
		Markdown:  []byte("![missing](/public/posts/hello/missing.png)"),
	}
	err := validateAssetReferences(post)
	if err == nil || !strings.Contains(err.Error(), "missing.png") {
		t.Fatalf("validateAssetReferences() error = %v, want missing asset error", err)
	}
}

func TestRenderPageEscapesMetadataButKeepsTrustedHTML(t *testing.T) {
	directory := t.TempDir()
	base := filepath.Join(directory, "base.gohtml")
	if err := writeTestFile(base, `<title>{{.Title}}</title><main>{{.Content}}</main>`); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "page.html")
	if err := RenderPage(base, Page{Title: `<script>alert("x")</script>`, Content: template.HTML(`<p>trusted</p>`)}, destination); err != nil {
		t.Fatalf("RenderPage() error = %v", err)
	}
	output := readTestFile(t, destination)
	if strings.Contains(output, "<script>") || !strings.Contains(output, "&lt;script&gt;") {
		t.Fatalf("RenderPage() did not escape title: %s", output)
	}
	if !strings.Contains(output, "<p>trusted</p>") {
		t.Fatalf("RenderPage() escaped trusted content: %s", output)
	}
}
