package ssg

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type Page struct {
	Title string
	Description string
	Content string
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

func GenerateBlogPage(posts []Post) {
	t, err := template.ParseFiles("templates/blog.gohtml")
	postList := &bytes.Buffer{}

	err = t.Execute(postList, posts)
	if err != nil {
		panic(err)
	}

	fmt.Print(postList.String())

	blogPage := Page {
		Title: "Vitor Almeida - Blog",
		Description: "Blog de Vitor Almeida",
		Content: postList.String(),
	}

	blogPage.generatePage("html/pages/blog.html")
}

func GenerateIndexPage(posts []Post) {
	t, err := template.ParseFiles("templates/home.gohtml")
	postList := &bytes.Buffer{}

	err = t.Execute(postList, posts)
	if err != nil {
		panic(err)
	}

	fmt.Print(postList.String())

	indexPage := Page {
		Title: "Vitor Almeida - Blog",
		Description: "Blog de Vitor Almeida",
		Content: postList.String(),
	}

	indexPage.generatePage("html/index.html")
}

func GeneratePostsPages(posts []Post) {
	t, err := template.ParseFiles("templates/post.gohtml")
	postPage := &bytes.Buffer{}

	for _, post := range(posts) {
		p := struct {
			Title string
			Date string
			Description string
			Content string
		} {
			Title: post.Title,
			Description: post.Description,
			Date: post.Date,
			Content: string(post.Content),
		}

		err = t.Execute(postPage, p)
		if err != nil {
			panic(err)
		}
		fmt.Print(postPage.String())

		postPage := Page {
			Title: post.Title,
			Description: post.Description,
			Content: postPage.String(),
		}
		postPage.generatePage(fmt.Sprintf("html/blog/%s.html", post.FileName))
	}
}

func GenerateImmutablePage(title, description, page string) {
	templateFile := fmt.Sprintf("templates/%s.gohtml", page)
	t, err := template.ParseFiles(templateFile)
	if err != nil {
		panic(err)
	}
	pcontent := &bytes.Buffer{}
	err = t.Execute(pcontent, nil)
	if err != nil {
		panic(err)
	}
	portfolioPage := Page {
		Title: title,
		Description: description,
		Content: pcontent.String(),
	}
	dest := fmt.Sprintf("html/pages/%s.html", page)
	portfolioPage.generatePage(dest)
}