package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/vitoraalmeida/ssg/ssg"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	postsPath := filepath.Join(cwd, "posts")
	posts := ssg.GetPosts(postsPath)

	createDestinationPaths()

	pl := ssg.GeneratePostListing(posts)
	ssg.GeneratePage(pl, "index", "Vitor Almeida", "Página pessoal de Vitor Almeida")
	ssg.GeneratePage(pl, "blog", "Vitor Almeida - Blog", "Blog de Vitor Almeida")
	ssg.GeneratePage("", "about", "Vitor Almeida - Sobre mim", "Página pessoal de Vitor Almeida")
	ssg.GeneratePage("", "portfolio", "Vitor Almeida - Portfólio", "Portfólio de Vitor Almeida")
	ssg.GeneratePostsPages(posts)
}

func createDestinationPaths() {
	blogPath := "src/blog"
	err := os.Mkdir(blogPath, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
	postsImagesDir := "src/public/posts_images"
	err = os.Mkdir(postsImagesDir, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
}

