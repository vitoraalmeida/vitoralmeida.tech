package ssg

import (
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/russross/blackfriday"
)

type Post struct {
	Title string
	Date string
	Description string
	Content []byte
	FileName string
}

type postMeta struct {
	Title string `toml:"title"`
	Date string `toml:"date"`
	Description string `toml:"description"`
}

type postLocation struct {
	name string
	content string
	meta string
}

func GetPosts(postsRootPath string ) (posts []Post){
	pls := getPostsLocations(postsRootPath)
	for _, pl := range pls {
		pm := getPostMeta(pl.meta)
		pc := getPostContentHtml(pl.content)
		post := Post{
			Title: pm.Title,
			Date: pm.Date,
			Description: pm.Description,
			Content: pc,
			FileName: pl.name,
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

	var pls []postLocation
	
	for _, post := range postsPaths {
		postDir := filepath.Join(postsRootPath, post.Name())
		var pl postLocation
		pl.name = post.Name()
		pl.meta = filepath.Join(postDir, "meta.toml")
		pl.content = filepath.Join(postDir, "post.md")
		pls = append(pls, pl)
	}
	return pls
}

func getPostMeta(path string) (pm postMeta) {
	if _,  err := toml.DecodeFile(path, &pm); err != nil {
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