package ssg

import (
	"fmt"
	"io/fs"
	"log"
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
	Content     []byte
	FileName    string
	ImagesDir   string
}

type postMeta struct {
	Title       string `toml:"title"`
	Date        string `toml:"date"`
	Description string `toml:"description"`
}

type postLocation struct {
	name    string
	content string
	meta    string
	images  string
}

func GetPosts(postsRootPath string) (posts []Post) {
	pls := getPostsLocations(postsRootPath)
	for _, pl := range pls {
		pm := getPostMeta(pl.meta)
		pc := getPostContentHtml(pl.content)
		post := Post{
			Title:       pm.Title,
			Date:        pm.Date,
			Description: pm.Description,
			Content:     pc,
			FileName:    pl.name,
			ImagesDir:   pl.images,
		}
		posts = append(posts, post)
	}
	return
}

func getPostsLocations(postsRootPath string) []postLocation {
	postsPaths, err := os.ReadDir(postsRootPath)
	if err != nil {
		log.Fatal(err)
	}
	// os diretórios dos posts são criados com uma numeração correspondente a ordem em que foram
	// escritos. Para mostrar no site na ordem de lançamento, invertemos a ordem de escrita
	sortFileNameDescend(postsPaths)

	var pls []postLocation

	for _, post := range postsPaths {
		postDir := filepath.Join(postsRootPath, post.Name())
		var pl postLocation
		pl.name = strings.SplitN(post.Name(), "-", 2)[1]
		pl.meta = filepath.Join(postDir, "meta.toml")
		pl.content = filepath.Join(postDir, "post.md")
		if imagesExists(filepath.Join(postsRootPath, post.Name())) {
			pl.images = filepath.Join(postDir, "images")
		}
		pls = append(pls, pl)
	}
	return pls
}

func getPostMeta(path string) (pm postMeta) {
	if _, err := toml.DecodeFile(path, &pm); err != nil {
		log.Fatal(err)
	}
	return
}

func getPostContentHtml(path string) []byte {
	input, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	html := blackfriday.MarkdownCommon(input)
	return html
}

func imagesExists(path string) bool {
	dirs, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, dir := range dirs {
		if dir.Name() == "images" {
			return true
		}
	}
	return false
}

func sortFileNameDescend(files []fs.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
}
