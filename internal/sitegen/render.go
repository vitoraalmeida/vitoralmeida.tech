package sitegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type Page struct {
	Title       string
	Description string
	Content     string
}

func renderSite(templatesDir, output string, posts []Post) error {
	listing, err := renderTemplate(filepath.Join(templatesDir, "post-listing.gohtml"), posts)
	if err != nil {
		return err
	}

	pages := []struct {
		name, title, description, nested string
	}{
		{"index", "Vitor Almeida", "Página pessoal de Vitor Almeida", listing},
		{"blog", "Vitor Almeida - Blog", "Blog de Vitor Almeida", listing},
		{"about", "Vitor Almeida - Sobre mim", "Página pessoal de Vitor Almeida", ""},
		{"portfolio", "Vitor Almeida - Portfólio", "Portfólio de Vitor Almeida", ""},
		{"404", "Vitor Almeida - Not found", "Fim da linha", ""},
	}
	for _, page := range pages {
		content, err := renderTemplate(filepath.Join(templatesDir, page.name+".gohtml"), page.nested)
		if err != nil {
			return err
		}
		if err := RenderPage(filepath.Join(templatesDir, "base-template.gohtml"), Page{
			Title: page.title, Description: page.description, Content: content,
		}, filepath.Join(output, page.name+".html")); err != nil {
			return err
		}
	}

	blogDir := filepath.Join(output, "blog")
	if err := os.MkdirAll(blogDir, 0o755); err != nil {
		return fmt.Errorf("create posts output directory: %w", err)
	}
	for _, post := range posts {
		content, err := renderTemplate(filepath.Join(templatesDir, "post.gohtml"), struct {
			Title, Date, Description, Content string
		}{post.Title, post.Date, post.Description, string(post.Content)})
		if err != nil {
			return err
		}
		if err := RenderPage(filepath.Join(templatesDir, "base-template.gohtml"), Page{
			Title: post.Title, Description: post.Description, Content: content,
		}, filepath.Join(blogDir, post.Slug+".html")); err != nil {
			return err
		}
		if post.ImagesDir != "" {
			if err := copyDirectoryContents(post.ImagesDir, filepath.Join(output, "public", "posts_images")); err != nil {
				return fmt.Errorf("copy images for post %q: %w", post.Slug, err)
			}
		}
	}
	return nil
}

func renderTemplate(path string, data any) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", path, err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", path, err)
	}
	return output.String(), nil
}

func RenderPage(baseTemplate string, page Page, destination string) error {
	tmpl, err := template.ParseFiles(baseTemplate)
	if err != nil {
		return fmt.Errorf("parse base template %q: %w", baseTemplate, err)
	}
	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create page %q: %w", destination, err)
	}
	if err := tmpl.Execute(file, page); err != nil {
		_ = file.Close()
		return fmt.Errorf("render page %q: %w", destination, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close page %q: %w", destination, err)
	}
	return nil
}
