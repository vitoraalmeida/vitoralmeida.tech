package ssg

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"
)

type Page struct {
	Title       string
	Description string
	Content     string
	Images      string
}

func (p Page) generatePage(destination string) {
	t, err := template.ParseFiles("templates/base-template.gohtml")
	if err != nil {
		panic(err)
	}

	pageFile, _ := os.Create(destination)
	defer pageFile.Close()
	err = t.Execute(pageFile, p)
	if err != nil {
		panic(err)
	}
}

func GeneratePage(nested, pageName, title, description string) {
	templateFile := fmt.Sprintf("templates/%s.gohtml", pageName)
	t, err := template.ParseFiles(templateFile)
	if err != nil {
		panic(err)
	}

	finalContent := &bytes.Buffer{}
	err = t.Execute(finalContent, nested)
	if err != nil {
		panic(err)
	}

	page := Page{
		Title:       title,
		Description: description,
		Content:     finalContent.String(),
	}
	dest := fmt.Sprintf("src/%s.html", pageName)
	page.generatePage(dest)
}

func GeneratePostsPages(posts []Post) {
	t, err := template.ParseFiles("templates/post.gohtml")
	var postPageContent *bytes.Buffer

	for _, post := range posts {
		postPageContent = &bytes.Buffer{}

		p := struct {
			Title       string
			Date        string
			Description string
			Content     string
		}{
			Title:       post.Title,
			Description: post.Description,
			Date:        post.Date,
			Content:     string(post.Content),
		}

		err = t.Execute(postPageContent, p)
		if err != nil {
			panic(err)
		}

		postPage := Page{
			Title:       post.Title,
			Description: post.Description,
			Content:     postPageContent.String(),
		}
		postPage.generatePage(fmt.Sprintf("src/blog/%s.html", post.FileName))
		copyPostImages(post)
	}
}

func copyPostImages(post Post) {
	dst := "src/public/posts_images"
	if len(post.ImagesDir) == 0 {
		fmt.Println("Post não tem imagens")
		return
	}
	imagesPaths, err := os.ReadDir(post.ImagesDir)
	if err != nil {
		panic(err)
	}
	for _, path := range imagesPaths {
		fin, err := os.Open(filepath.Join(post.ImagesDir, path.Name()))
		if err != nil {
			log.Fatal(err)
		}

		defer fin.Close()

		fout, err := os.Create(filepath.Join(dst, path.Name()))
		if err != nil {
			log.Fatal(err)
		}
		defer fout.Close()

		_, err = io.Copy(fout, fin)

		if err != nil {
			log.Fatal(err)
		}
	}
}
