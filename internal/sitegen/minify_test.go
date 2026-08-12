package sitegen

import "testing"

func TestMinifyHTMLPreservesRawElements(t *testing.T) {
	input := []byte("<main>\n  <p>One</p>\n  <pre>  keep\n  spacing</pre>\n  <script>const value = \"a  b\";</script>\n</main>")
	got := string(minifyHTML(input))
	want := `<main> <p>One</p> <pre>  keep
  spacing</pre> <script>const value = "a  b";</script> </main>`
	if got != want {
		t.Fatalf("minifyHTML() = %q, want %q", got, want)
	}
}

func TestMinifyCSSPreservesStringsAndValueSpaces(t *testing.T) {
	input := []byte("/* comment */\n.card {\n  content: \"a  b\";\n  margin: 0 auto;\n}\n")
	got := string(minifyCSS(input))
	want := `.card{content:"a  b";margin:0 auto;}`
	if got != want {
		t.Fatalf("minifyCSS() = %q, want %q", got, want)
	}
}
