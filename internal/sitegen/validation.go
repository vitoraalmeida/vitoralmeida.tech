package sitegen

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	postDirectoryPattern  = regexp.MustCompile(`^[0-9]+-[a-z0-9]+(?:-[a-z0-9]+)*$`)
	assetReferencePattern = regexp.MustCompile(`\]\((/public/posts/([^/\s)]+)/([^\s)]+))`)
)

var requiredTemplates = []string{
	"base-template.gohtml", "post-listing.gohtml", "post.gohtml", "index.gohtml",
	"blog.gohtml", "about.gohtml", "portfolio.gohtml", "404.gohtml",
}

// check executa todas as validações de entrada — diretórios, templates, posts
// e colisões de destino — retornando os posts carregados quando tudo é válido.
func check(config Config) ([]Post, error) {
	if err := validateInputs(config); err != nil {
		return nil, err
	}
	if err := validateTemplates(config.TemplatesDir); err != nil {
		return nil, err
	}
	posts, err := LoadPosts(filepath.Join(config.ContentDir, "posts"))
	if err != nil {
		return nil, err
	}
	if err := validatePosts(posts); err != nil {
		return nil, err
	}
	if err := validateDestinations(config.StaticDir, posts); err != nil {
		return nil, err
	}
	return posts, nil
}

// validateTemplates garante que todos os templates obrigatórios existem, são
// arquivos regulares e compilam, falhando cedo antes de qualquer renderização.
func validateTemplates(directory string) error {
	for _, name := range requiredTemplates {
		path := filepath.Join(directory, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("validate required template %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("validate required template %q: not a regular file", path)
		}
		if _, err := template.ParseFiles(path); err != nil {
			return fmt.Errorf("validate required template %q: %w", path, err)
		}
	}
	return nil
}

// validatePosts valida os metadados e estrutura de cada post: nome de
// diretório no padrão NN-slug, slug único e obrigatório, title/description
// presentes, data em DD/MM/YYYY, referências de assets e diretório de assets.
func validatePosts(posts []Post) error {
	seenSlugs := make(map[string]string, len(posts))
	for _, post := range posts {
		directoryName := filepath.Base(post.SourceDir)
		if !postDirectoryPattern.MatchString(directoryName) {
			return fmt.Errorf("validate post %q: directory name must match NN-slug using lowercase letters, numbers, and hyphens", directoryName)
		}
		if post.Slug == "" {
			return fmt.Errorf("validate post %q: slug is required", directoryName)
		}
		if previous, exists := seenSlugs[post.Slug]; exists {
			return fmt.Errorf("validate post %q: slug %q duplicates post %q", directoryName, post.Slug, previous)
		}
		seenSlugs[post.Slug] = directoryName
		if strings.TrimSpace(post.Title) == "" {
			return fmt.Errorf("validate post %q metadata: field %q is required", directoryName, "title")
		}
		if strings.TrimSpace(post.Description) == "" {
			return fmt.Errorf("validate post %q metadata: field %q is required", directoryName, "description")
		}
		if _, err := time.Parse("02/01/2006", post.Date); err != nil {
			return fmt.Errorf("validate post %q metadata: field %q must use DD/MM/YYYY: %w", directoryName, "date", err)
		}
		if err := validateAssetReferences(post); err != nil {
			return err
		}
		if post.AssetsDir != "" {
			info, err := os.Stat(post.AssetsDir)
			if err != nil {
				return fmt.Errorf("validate post %q assets: %w", directoryName, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("validate post %q assets: not a directory", directoryName)
			}
		}
	}
	return nil
}

// validateDestinations garante que nenhum arquivo gerado colida com outro
// (páginas, posts, assets e estáticos), pois sobrescreveria arquivos na saída.
func validateDestinations(staticDir string, posts []Post) error {
	generated := map[string]string{
		"index.html": "index page", "blog.html": "blog page", "about.html": "about page",
		"portfolio.html": "portfolio page", "404.html": "404 page",
	}
	for _, post := range posts {
		generated[filepath.Join("blog", post.Slug+".html")] = fmt.Sprintf("post %q", post.Slug)
		if post.AssetsDir == "" {
			continue
		}
		err := filepath.WalkDir(post.AssetsDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			relative, err := filepath.Rel(post.AssetsDir, path)
			if err != nil {
				return err
			}
			destination := filepath.Join("public", "posts", post.Slug, relative)
			if previous, exists := generated[destination]; exists {
				return fmt.Errorf("duplicate destination %q from post %q asset and %s", destination, post.Slug, previous)
			}
			generated[destination] = fmt.Sprintf("post %q asset", post.Slug)
			return nil
		})
		if err != nil {
			return err
		}
	}

	return filepath.WalkDir(staticDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(staticDir, path)
		if err != nil {
			return err
		}
		if previous, exists := generated[relative]; exists {
			return fmt.Errorf("duplicate destination %q from static file and %s", relative, previous)
		}
		return nil
	})
}

// validateAssetReferences confere que cada imagem referenciada no Markdown usa
// o slug do próprio post, aponta para um caminho seguro e existe como arquivo.
func validateAssetReferences(post Post) error {
	for _, match := range assetReferencePattern.FindAllSubmatch(post.Markdown, -1) {
		publicPath, slug, asset := string(match[1]), string(match[2]), string(match[3])
		if slug != post.Slug {
			return fmt.Errorf("validate post %q: asset reference %q uses slug %q", post.Slug, publicPath, slug)
		}
		cleanAsset := filepath.Clean(filepath.FromSlash(asset))
		if cleanAsset == "." || filepath.IsAbs(cleanAsset) || cleanAsset == ".." || strings.HasPrefix(cleanAsset, ".."+string(filepath.Separator)) {
			return fmt.Errorf("validate post %q: unsafe asset reference %q", post.Slug, publicPath)
		}
		path := filepath.Join(post.AssetsDir, cleanAsset)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("validate post %q asset %q: %w", post.Slug, publicPath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("validate post %q asset %q: not a regular file", post.Slug, publicPath)
		}
	}
	return nil
}
