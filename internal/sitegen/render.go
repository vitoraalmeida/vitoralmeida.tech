package sitegen

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

const siteBaseURL = "https://vitoralmeida.tech"

type Page struct {
	Title          string
	Description    string
	ActiveSection  string
	Content        template.HTML
	Canonical      string
	Type           string
	DateISO        string
	OGImage        string
	NoIndex        bool
	StructuredData template.HTML
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

// renderSite gera todo o HTML do site (páginas fixas, posts, RSS e sitemap)
// no diretório de saída, usando os posts já carregados e ordenados. Quando
// noIndex é verdadeiro, todas as páginas recebem a meta tag noindex.
func renderSite(templatesDir, output string, posts []Post, noIndex bool) error {
	listing, err := renderTemplate(filepath.Join(templatesDir, "post-listing.gohtml"), posts)
	if err != nil {
		return err
	}

	pages := []struct {
		name, title, description, activeSection, canonical, kind string
		nested                                                    template.HTML
	}{
		{"index", "Vitor Almeida", "Engenheiro de segurança da informação. Compartilho artigos sobre tecnologia, segurança de servidores, privacidade e desenvolvimento.", "home", "/", "website", listing},
		{"blog", "Vitor Almeida - Blog", "Artigos sobre tecnologia, segurança da informação, privacidade e desenvolvimento.", "blog", "/blog", "webpage", listing},
		{"about", "Vitor Almeida - Sobre mim", "Quem sou, o que faço e como me encontrar.", "about", "/about", "webpage", ""},
		{"portfolio", "Vitor Almeida - Portfólio", "Portfólio de Vitor Almeida", "portfolio", "/portfolio", "webpage", ""},
		{"404", "Vitor Almeida - Not found", "Fim da linha", "", "", "webpage", ""},
	}
	for _, page := range pages {
		content, err := renderTemplate(filepath.Join(templatesDir, page.name+".gohtml"), page.nested)
		if err != nil {
			return err
		}
		canonical := ""
		ogImage := ""
		if page.canonical != "" {
			canonical = siteBaseURL + page.canonical
			ogImage = siteBaseURL + "/og-image.png"
		}
		if err := RenderPage(filepath.Join(templatesDir, "base-template.gohtml"), Page{
			Title: page.title, Description: page.description, ActiveSection: page.activeSection, Content: content,
			Canonical: canonical, Type: page.kind, OGImage: ogImage, NoIndex: noIndex,
			StructuredData: pageStructuredData(page.kind, page.title, page.description, canonical, ""),
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
		canonical := siteBaseURL + "/blog/" + post.Slug
		if err := RenderPage(filepath.Join(templatesDir, "base-template.gohtml"), Page{
			Title: post.Title, Description: post.Description, ActiveSection: "blog", Content: content,
			Canonical: canonical, Type: "article", DateISO: post.DateISO, OGImage: siteBaseURL + "/og-image.png",
			NoIndex: noIndex,
			StructuredData: pageStructuredData("article", post.Title, post.Description, canonical, post.DateISO),
		}, filepath.Join(blogDir, post.Slug+".html")); err != nil {
			return err
		}
	}
	if err := writeRSS(output, posts); err != nil {
		return err
	}
	return writeSitemap(output, posts)
}

// renderTemplate executa um único arquivo de template Go com os dados
// fornecidos e devolve o HTML resultante. Retorna template.HTML porque os
// templates do repositório são HTML confiável e não devem ser re-escapados.
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

// RenderPage monta uma página completa a partir do template base e grava o
// resultado no destino, retornando erro contextual em cada etapa da escrita.
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

// pageStructuredData gera o JSON-LD (schema.org) para a página, retornando
// HTML vazio quando não há URL canônica, evitando indexar páginas inválidas.
func pageStructuredData(kind, title, description, url, dateISO string) template.HTML {
	if url == "" {
		return ""
	}
	author := map[string]any{
		"@type":     "Person",
		"name":      "Vitor Almeida",
		"url":       siteBaseURL + "/about",
		"jobTitle":  "Application Security Engineer",
		"sameAs":    []string{"https://github.com/vitoraalmeida", "https://www.linkedin.com/in/vitoralmeida00/"},
	}
	var schema map[string]any
	switch kind {
	case "website":
		schema = map[string]any{
			"@context":   "https://schema.org",
			"@type":      "WebSite",
			"name":       title,
			"url":        url,
			"inLanguage": "pt-BR",
			"author":     author,
			"potentialAction": map[string]any{
				"@type":       "SearchAction",
				"target":      siteBaseURL + "/?q={search_term_string}",
				"query-input": "required name=search_term_string",
			},
		}
	case "article":
		schema = map[string]any{
			"@context":         "https://schema.org",
			"@type":            "BlogPosting",
			"headline":         title,
			"description":      description,
			"url":              url,
			"inLanguage":       "pt-BR",
			"author":           author,
			"publisher":        author,
			"image":            siteBaseURL + "/og-image.png",
			"mainEntityOfPage": map[string]any{"@type": "WebPage", "@id": url},
		}
		if dateISO != "" {
			schema["datePublished"] = dateISO
			schema["dateModified"] = dateISO
		}
	default:
		schema = map[string]any{
			"@context":    "https://schema.org",
			"@type":       "WebPage",
			"name":        title,
			"description": description,
			"url":         url,
			"inLanguage":  "pt-BR",
		}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return ""
	}
	return template.HTML(`<script type="application/ld+json">` + string(encoded) + `</script>`)
}

// writeSitemap gera o sitemap.xml com as páginas fixas e todos os posts,
// usando a data ISO do post mais recente como lastmod para páginas dinâmicas.
func writeSitemap(output string, posts []Post) error {
	latestPostDate := ""
	if len(posts) > 0 {
		latestPostDate = posts[0].DateISO
	}
	entries := []struct{ path, lastmod string }{
		{"/", latestPostDate},
		{"/blog", latestPostDate},
		{"/about", latestPostDate},
		{"/portfolio", latestPostDate},
		{"/feed.xml", latestPostDate},
	}
	for _, post := range posts {
		entries = append(entries, struct{ path, lastmod string }{"/blog/" + post.Slug, post.DateISO})
	}

	var buffer bytes.Buffer
	buffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buffer.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, entry := range entries {
		buffer.WriteString("  <url>\n")
		buffer.WriteString("    <loc>" + siteBaseURL + entry.path + "</loc>\n")
		if entry.lastmod != "" {
			buffer.WriteString("    <lastmod>" + entry.lastmod + "</lastmod>\n")
		}
		buffer.WriteString("  </url>\n")
	}
	buffer.WriteString("</urlset>\n")
	return os.WriteFile(filepath.Join(output, "sitemap.xml"), buffer.Bytes(), 0o644)
}

// writeRSS gera o feed RSS 2.0 com o conteúdo completo de cada post.
func writeRSS(output string, posts []Post) error {
	var buffer bytes.Buffer
	buffer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buffer.WriteString(`<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n")
	buffer.WriteString("<channel>\n")
	buffer.WriteString("  <title>Vitor Almeida</title>\n")
	buffer.WriteString("  <link>" + siteBaseURL + "/</link>\n")
	buffer.WriteString("  <description>Engenheiro de segurança da informação. Compartilho artigos sobre tecnologia, segurança de servidores, privacidade e desenvolvimento.</description>\n")
	buffer.WriteString("  <language>pt-br</language>\n")
	buffer.WriteString("  <atom:link href=\"" + siteBaseURL + "/feed.xml\" rel=\"self\" type=\"application/rss+xml\" />\n")
	buffer.WriteString("  <lastBuildDate>" + time.Now().UTC().Format(time.RFC1123Z) + "</lastBuildDate>\n")

	for _, post := range posts {
		buffer.WriteString("  <item>\n")
		buffer.WriteString("    <title>" + rssEscape(post.Title) + "</title>\n")
		buffer.WriteString("    <link>" + siteBaseURL + "/blog/" + post.Slug + "</link>\n")
		buffer.WriteString("    <guid isPermaLink=\"true\">" + siteBaseURL + "/blog/" + post.Slug + "</guid>\n")
		if pubDate, err := time.Parse("2006-01-02", post.DateISO); err == nil {
			buffer.WriteString("    <pubDate>" + pubDate.Format(time.RFC1123Z) + "</pubDate>\n")
		}
		content := "<![CDATA[" + string(post.Content) + "]]>"
		buffer.WriteString("    <description>" + content + "</description>\n")
		buffer.WriteString("    <content:encoded>" + content + "</content:encoded>\n")
		buffer.WriteString("    <author>vitor@vitoralmeida.tech (Vitor Almeida)</author>\n")
		buffer.WriteString("  </item>\n")
	}
	buffer.WriteString("</channel>\n</rss>\n")
	return os.WriteFile(filepath.Join(output, "feed.xml"), buffer.Bytes(), 0o644)
}

// rssEscape escapa caracteres especiais para o conteúdo textual do XML.
func rssEscape(value string) string {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		return value
	}
	return buffer.String()
}
