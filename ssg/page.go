package ssg

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type Page struct {
	Title       string
	Description string
	Content     string
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
	postPageContent := &bytes.Buffer{}

	for _, post := range posts {
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
		fmt.Print(postPageContent.String())

		postPage := Page{
			Title:       post.Title,
			Description: post.Description,
			Content:     postPageContent.String(),
		}
		postPage.generatePage(fmt.Sprintf("src/blog/%s.html", post.FileName))
	}
}

