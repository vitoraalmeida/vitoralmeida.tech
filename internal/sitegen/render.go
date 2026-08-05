package sitegen

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

type Page struct {
	Title         string
	Description   string
	ActiveSection string
	Content       template.HTML
}

type PostLink struct {
	Title string
	Slug  string
}

type PostPage struct {
	Title           string
	Date            string
	DateISO         string
	Description     string
	Content         template.HTML
	TableOfContents []TOCItem
	Older           *PostLink
	Newer           *PostLink
}

func renderSite(templatesDir, output string, posts []Post) error {
	listing, err := renderTemplate(filepath.Join(templatesDir, "post-listing.gohtml"), posts)
	if err != nil {
		return err
	}

	pages := []struct {
		name, title, description, activeSection string
		nested                                  template.HTML
	}{
		{"index", "Vitor Almeida", "Página pessoal de Vitor Almeida", "home", listing},
		{"blog", "Vitor Almeida - Blog", "Blog de Vitor Almeida", "blog", listing},
		{"about", "Vitor Almeida - Sobre mim", "Página pessoal de Vitor Almeida", "about", ""},
		{"portfolio", "Vitor Almeida - Portfólio", "Portfólio de Vitor Almeida", "portfolio", ""},
		{"404", "Vitor Almeida - Not found", "Fim da linha", "", ""},
	}
	for _, page := range pages {
		content, err := renderTemplate(filepath.Join(templatesDir, page.name+".gohtml"), page.nested)
		if err != nil {
			return err
		}
		if err := RenderPage(filepath.Join(templatesDir, "base-template.gohtml"), Page{
			Title: page.title, Description: page.description, ActiveSection: page.activeSection, Content: content,
		}, filepath.Join(output, page.name+".html")); err != nil {
			return err
		}
	}

	blogDir := filepath.Join(output, "blog")
	if err := os.MkdirAll(blogDir, 0o755); err != nil {
		return fmt.Errorf("create posts output directory: %w", err)
	}
	for index, post := range posts {
		postPage := PostPage{
			Title:           post.Title,
			Date:            post.Date,
			DateISO:         post.DateISO,
			Description:     post.Description,
			Content:         post.Content,
			TableOfContents: post.TableOfContents,
		}
		if index+1 < len(posts) {
			postPage.Older = &PostLink{Title: posts[index+1].Title, Slug: posts[index+1].Slug}
		}
		if index > 0 {
			postPage.Newer = &PostLink{Title: posts[index-1].Title, Slug: posts[index-1].Slug}
		}

		content, err := renderTemplate(filepath.Join(templatesDir, "post.gohtml"), postPage)
		if err != nil {
			return err
		}
		if err := RenderPage(filepath.Join(templatesDir, "base-template.gohtml"), Page{
			Title: post.Title, Description: post.Description, ActiveSection: "blog", Content: content,
		}, filepath.Join(blogDir, post.Slug+".html")); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplate(path string, data any) (template.HTML, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", path, err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", path, err)
	}
	return template.HTML(output.String()), nil // Repository templates are trusted HTML.
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
