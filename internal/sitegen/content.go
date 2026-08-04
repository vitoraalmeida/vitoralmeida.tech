package sitegen

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/russross/blackfriday"
)

type Post struct {
	Title       string
	Date        string
	Description string
	Content     template.HTML
	Markdown    []byte
	Slug        string
	AssetsDir   string
	SourceDir   string
}

type postMeta struct {
	Title       string `toml:"title"`
	Date        string `toml:"date"`
	Description string `toml:"description"`
}

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

	return Post{
		Title:       metadata.Title,
		Date:        metadata.Date,
		Description: metadata.Description,
		Content:     template.HTML(blackfriday.MarkdownCommon(markdown)), // Markdown files are trusted repository content.
		Markdown:    markdown,
		Slug:        parts[1],
		AssetsDir:   assetsDir,
		SourceDir:   postDir,
	}, nil
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
