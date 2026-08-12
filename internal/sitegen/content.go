package sitegen

import (
	"fmt"
	"html"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/russross/blackfriday"
)

type Post struct {
	Title           string
	Date            string
	DateISO         string
	Description     string
	Content         template.HTML
	TableOfContents []TOCItem
	Markdown        []byte
	Slug            string
	AssetsDir       string
	SourceDir       string
}

type TOCItem struct {
	ID       string
	Title    string
	Children []TOCItem
}

var (
	standaloneImageParagraphPattern = regexp.MustCompile(`<p>(<img src="[^"]*" alt="([^"]*)"(?: title="[^"]*")? />)</p>`)
	imgTagPattern                   = regexp.MustCompile(`<img[^>]*>`)
	headingPattern                  = regexp.MustCompile(`(?s)<h([23]) id="([^"]+)">(.*?)</h[23]>`)
	htmlTagPattern                  = regexp.MustCompile(`<[^>]+>`)
)

type postMeta struct {
	Title       string `toml:"title"`
	Date        string `toml:"date"`
	Description string `toml:"description"`
}

// LoadPosts lê todos os posts do diretório raiz, ordenando-os do mais recente
// para o mais antigo, já com Markdown renderizado e slug extraído do nome.
func LoadPosts(postsRoot string) ([]Post, error) {
	entries, err := os.ReadDir(postsRoot)
	if err != nil {
		return nil, fmt.Errorf("read posts directory %q: %w", postsRoot, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })

	posts := make([]Post, 0, len(entries))
	for _, entry := range entries {
		post, err := loadPost(postsRoot, entry)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

// loadPost carrega um único post: meta.toml (metadados), post.md (conteúdo)
// e a pasta assets (imagens). A data em DD/MM/YYYY é convertida para ISO para
// uso em sitemap e structured data, sem falhar se o formato for inesperado.
func loadPost(postsRoot string, entry fs.DirEntry) (Post, error) {
	if !entry.IsDir() {
		return Post{}, fmt.Errorf("load post %q: expected a directory", entry.Name())
	}
	parts := strings.SplitN(entry.Name(), "-", 2)
	if len(parts) != 2 || parts[1] == "" {
		return Post{}, fmt.Errorf("load post %q: directory name must match NN-slug", entry.Name())
	}

	postDir := filepath.Join(postsRoot, entry.Name())
	metaPath := filepath.Join(postDir, "meta.toml")
	var metadata postMeta
	if _, err := toml.DecodeFile(metaPath, &metadata); err != nil {
		return Post{}, fmt.Errorf("load post %q metadata: %w", entry.Name(), err)
	}
	markdownPath := filepath.Join(postDir, "post.md")
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		return Post{}, fmt.Errorf("load post %q content: %w", entry.Name(), err)
	}

	assetsDir := filepath.Join(postDir, "assets")
	if _, err := os.Stat(assetsDir); errorsIsNotExist(err) {
		assetsDir = ""
	} else if err != nil {
		return Post{}, fmt.Errorf("inspect post %q assets: %w", entry.Name(), err)
	}

	dateISO := ""
	if date, err := time.Parse("02/01/2006", metadata.Date); err == nil {
		dateISO = date.Format("2006-01-02")
	}
	content, tableOfContents := renderMarkdown(markdown, assetsDir)

	return Post{
		Title:           metadata.Title,
		Date:            metadata.Date,
		DateISO:         dateISO,
		Description:     metadata.Description,
		Content:         content,
		TableOfContents: tableOfContents,
		Markdown:        markdown,
		Slug:            parts[1],
		AssetsDir:       assetsDir,
		SourceDir:       postDir,
	}, nil
}

// renderMarkdown converte Markdown em HTML com blackfriday e depois aplica as
// transformações de artigo: imagens isoladas viram figure, imagens subsequentes
// recebem lazy loading, headings ganham permalink e um TOC é construído.
// Dimensões de imagens são injetadas quando os arquivos existem em assetsDir.
func renderMarkdown(markdown []byte, assetsDir string) (template.HTML, []TOCItem) {
	htmlFlags := blackfriday.HTML_USE_XHTML |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_FRACTIONS |
		blackfriday.HTML_SMARTYPANTS_DASHES |
		blackfriday.HTML_SMARTYPANTS_LATEX_DASHES
	extensions := blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_AUTO_HEADER_IDS |
		blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
		blackfriday.EXTENSION_DEFINITION_LISTS
	renderer := blackfriday.HtmlRenderer(htmlFlags, "", "")
	rendered := string(blackfriday.MarkdownOptions(markdown, renderer, blackfriday.Options{Extensions: extensions}))
	rendered = standaloneImageParagraphPattern.ReplaceAllStringFunc(rendered, func(paragraph string) string {
		matches := standaloneImageParagraphPattern.FindStringSubmatch(paragraph)
		return `<figure class="article-figure">` + matches[1] +
			`<figcaption class="article-figure__caption">` + matches[2] + `</figcaption></figure>`
	})
	rendered = addImageDimensions(rendered, assetsDir)
	rendered = wrapWithPicture(rendered, assetsDir)
	rendered = addLazyLoadingToImages(rendered)
	tableOfContents := buildTableOfContents(rendered)
	rendered = addHeadingPermalinks(rendered)
	return template.HTML(rendered), tableOfContents // Repository Markdown and generated article markup are trusted HTML.
}

// addLazyLoadingToImages adiciona loading="lazy" a todas as imagens exceto a
// primeira do artigo, preservando a imagem acima da dobra para o LCP. A
// primeira imagem também recebe fetchpriority="high" para antecipar o LCP.
func addLazyLoadingToImages(rendered string) string {
	firstImage := true
	return imgTagPattern.ReplaceAllStringFunc(rendered, func(img string) string {
		if firstImage {
			firstImage = false
			if !strings.Contains(img, "fetchpriority=") {
				if index := strings.LastIndex(img, "/>"); index >= 0 {
					img = strings.TrimRight(img[:index], " ") + ` fetchpriority="high"` + img[index:]
				}
			}
			return img
		}
		if strings.Contains(img, "loading=") {
			return img
		}
		index := strings.LastIndex(img, "/>")
		if index < 0 {
			return img
		}
		return strings.TrimRight(img[:index], " ") + ` loading="lazy"` + img[index:]
	})
}

// responsiveWidths lista as larguras alvo das variantes responsivas WebP
// geradas quando a imagem original é maior que cada largura.
var responsiveWidths = []int{480, 800, 1280}

// wrapWithPicture converte cada <img> em <picture> com um <source> WebP quando
// as variantes otimizadas existem no diretório de assets do post; caso
// contrário mantém o <img> original como fallback para navegadores antigos.
func wrapWithPicture(rendered string, assetsDir string) string {
	if assetsDir == "" {
		return rendered
	}
	return imgTagPattern.ReplaceAllStringFunc(rendered, func(img string) string {
		ok, source := webpSource(img, assetsDir)
		if !ok {
			return img
		}
		return "<picture>" + source + img + "</picture>"
	})
}

// webpSource monta a tag <source type="image/webp"> para um <img> cujo caminho
// é /public/posts/{slug}/{resto}, usando srcset responsivo quando as variantes
// de largura existem. Retorna false quando não há variante WebP disponível.
func webpSource(img string, assetsDir string) (bool, string) {
	srcMatch := regexp.MustCompile(`src="([^"]+)"`).FindStringSubmatch(img)
	if len(srcMatch) < 2 {
		return false, ""
	}
	filename := strings.TrimPrefix(srcMatch[1], "/public/posts/")
	parts := strings.SplitN(filename, "/", 2)
	if len(parts) != 2 {
		return false, ""
	}
	dir, base := filepath.Split(parts[1])
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	baseDir := filepath.Join(assetsDir, dir)
	dirURL := "/public/posts/" + parts[0] + "/" + dir

	if _, err := os.Stat(filepath.Join(baseDir, stem+".webp")); err != nil {
		return false, ""
	}
	candidates := make([]string, 0, len(responsiveWidths))
	for _, width := range responsiveWidths {
		name := fmt.Sprintf("%s-%dw.webp", stem, width)
		if info, err := os.Stat(filepath.Join(baseDir, name)); err == nil && info.Mode().IsRegular() {
			candidates = append(candidates, fmt.Sprintf("%s%s %dw", dirURL, name, width))
		}
	}
	if len(candidates) > 0 {
		return true, `<source type="image/webp" srcset="` + strings.Join(candidates, ", ") +
			`" sizes="(max-width: 700px) 100vw, 70ch" />`
	}
	return true, `<source type="image/webp" srcset="` + dirURL + stem + `.webp" />`
}

// addImageDimensions injeta width e height nas tags img quando o arquivo
// existe no diretório de assets, evitando layout shift (CLS).
func addImageDimensions(rendered string, assetsDir string) string {
	if assetsDir == "" {
		return rendered
	}
	return imgTagPattern.ReplaceAllStringFunc(rendered, func(img string) string {
		if strings.Contains(img, "width=") && strings.Contains(img, "height=") {
			return img
		}
		srcMatch := regexp.MustCompile(`src="([^"]+)"`).FindStringSubmatch(img)
		if len(srcMatch) < 2 {
			return img
		}
		src := srcMatch[1]
		if !strings.HasPrefix(src, "/public/posts/") {
			return img
		}
		filename := strings.TrimPrefix(src, "/public/posts/")
		parts := strings.SplitN(filename, "/", 2)
		if len(parts) != 2 {
			return img
		}
		imagePath := filepath.Join(assetsDir, parts[1])
		width, height, err := getImageDimensions(imagePath)
		if err != nil {
			return img
		}
		index := strings.LastIndex(img, "/>")
		if index < 0 {
			return img
		}
		return strings.TrimRight(img[:index], " ") +
			fmt.Sprintf(` width="%d" height="%d"`, width, height) + img[index:]
	})
}

// getImageDimensions lê as dimensões de um arquivo de imagem, retornando
// erro se o arquivo não existir ou não for um formato suportado.
func getImageDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

// buildTableOfContents extrai os headings h2/h3 do HTML, aninhando h3 sob o
// h2 anterior. Retorna nil quando há menos de 3 headings para omitir o TOC.
func buildTableOfContents(rendered string) []TOCItem {
	headings := headingPattern.FindAllStringSubmatch(rendered, -1)
	if len(headings) < 3 {
		return nil
	}

	items := make([]TOCItem, 0, len(headings))
	for _, heading := range headings {
		item := TOCItem{
			ID:    heading[2],
			Title: html.UnescapeString(htmlTagPattern.ReplaceAllString(heading[3], "")),
		}
		if heading[1] == "3" && len(items) > 0 {
			items[len(items)-1].Children = append(items[len(items)-1].Children, item)
			continue
		}
		items = append(items, item)
	}
	return items
}

// addHeadingPermalinks injeta um link âncora (#) ao lado do texto de cada
// heading para que leitores possam compartilhar URLs diretas para a seção.
func addHeadingPermalinks(rendered string) string {
	return headingPattern.ReplaceAllStringFunc(rendered, func(heading string) string {
		matches := headingPattern.FindStringSubmatch(heading)
		level, id, content := matches[1], matches[2], matches[3]
		title := html.UnescapeString(htmlTagPattern.ReplaceAllString(content, ""))
		return `<h` + level + ` id="` + id + `">` + content +
			`<a class="heading-permalink" href="#` + id + `" aria-label="Link permanente para ` +
			template.HTMLEscapeString(title) + `">#</a></h` + level + `>`
	})
}

// errorsIsNotExist reporta se err representa um caminho inexistente, usado
// para tratar a ausência de assets como estado válido em vez de erro.
func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
