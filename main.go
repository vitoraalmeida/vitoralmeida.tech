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

	path := "html/pages"
	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
	blogPath := "html/blog"
	err = os.MkdirAll(blogPath, os.ModePerm)
	if err != nil {
		log.Println(err)
	}

	ssg.GenerateBlogPage(posts)
	ssg.GenerateIndexPage(posts)
	ssg.GeneratePostsPages(posts)
	ssg.GenerateAboutPage()
	ssg.GeneratePortfolioPage()
	os.Rename("styles", "html/styles")
	os.Rename("public", "html/public")
}