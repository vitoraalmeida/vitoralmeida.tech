package ssg

import (
	"bytes"
	"text/template"
)

func GeneratePostListing(posts []Post) string {
	t, err := template.ParseFiles("templates/post-listing.gohtml")
	postListing := &bytes.Buffer{}

	err = t.Execute(postListing, posts)
	if err != nil {
		panic(err)
	}
	return postListing.String()
}