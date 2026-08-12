package sitegen

import (
	"bytes"
	"regexp"
	"strings"
)

// tagNamePattern extrai o nome de uma tag a partir da abertura "<...".
var tagNamePattern = regexp.MustCompile(`^/?([a-zA-Z][a-zA-Z0-9]*)`)

// rawHTMLElements agrupa as tags cujo conteúdo literal não pode ser minificado
// (espaços em branco significativos, CSS ou JavaScript).
var rawHTMLElements = map[string]bool{
	"pre": true, "code": true, "script": true, "style": true, "textarea": true,
}

// minifyHTML colapsa runs de espaço em branco do HTML renderizado entre tags,
// preservando conteúdo literal dentro de pre/code/script/style/textarea e o
// texto de comentários <!-- -->.
func minifyHTML(html []byte) []byte {
	var out bytes.Buffer
	n := len(html)
	i := 0
	for i < n {
		switch {
		case html[i] == '<':
			switch {
			case bytes.HasPrefix(html[i:], []byte("<!--")):
				if end := bytes.Index(html[i+4:], []byte("-->")); end >= 0 {
					end += i + 4 + 3
					out.Write(html[i:end])
					i = end
					continue
				}
				out.Write(html[i:])
				i = n
			default:
				rel := bytes.IndexByte(html[i:], '>')
				if rel < 0 {
					out.Write(html[i:])
					i = n
					continue
				}
				end := i + rel + 1
				tag := html[i:end]
				out.Write(tag)
				name := tagNamePattern.FindSubmatch(bytes.TrimSpace(tag[1:]))
				if len(name) == 2 && rawHTMLElements[strings.ToLower(string(name[1]))] && tag[1] != '/' {
					if closeRel := bytes.Index(bytes.ToLower(html[end:]), []byte("</"+strings.ToLower(string(name[1])))); closeRel >= 0 {
						closeEnd := end + closeRel + 2 + len(name[1])
						if closeEnd < n && html[closeEnd] == '>' {
							closeEnd++
						}
						out.Write(html[end:closeEnd])
						i = closeEnd
						continue
					}
					out.Write(html[end:])
					i = n
					continue
				}
				i = end
			}
		case isHTMLSpace(html[i]):
			for i < n && isHTMLSpace(html[i]) {
				i++
			}
			out.WriteByte(' ')
		default:
			start := i
			for i < n && !isHTMLSpace(html[i]) && html[i] != '<' {
				i++
			}
			out.Write(html[start:i])
		}
	}
	return out.Bytes()
}

// isHTMLSpace reporta se o byte representa espaço em branco colapsável.
func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// minifyCSS remove comentários e colapsa espaço em branco ao redor de
// delimitadores, preservando strings e o espaço entre valores ("margin: 0 auto").
func minifyCSS(css []byte) []byte {
	var out bytes.Buffer
	n := len(css)
	i := 0
	pendingSpace := false
	writeToken := func(c byte) {
		if pendingSpace {
			previous := byte(0)
			if out.Len() > 0 {
				previous = out.Bytes()[out.Len()-1]
			}
			if !isCSSDelimiter(previous) && !isCSSDelimiter(c) {
				out.WriteByte(' ')
			}
			pendingSpace = false
		}
		out.WriteByte(c)
	}
	for i < n {
		c := css[i]
		switch {
		case c == '/' && i+1 < n && css[i+1] == '*':
			pendingSpace = true
			if end := bytes.Index(css[i+2:], []byte("*/")); end >= 0 {
				i += 2 + end + 2
			} else {
				i = n
			}
		case c == '"' || c == '\'':
			writeToken(c)
			start := i
			i++
			for i < n && css[i] != c {
				if css[i] == '\\' {
					i++
				}
				i++
			}
			i++
			out.Write(css[start+1 : i])
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			pendingSpace = true
			i++
		default:
			writeToken(c)
			i++
		}
	}
	return bytes.TrimSpace(out.Bytes())
}

func isCSSDelimiter(b byte) bool {
	return b == 0 || b == '{' || b == '}' || b == ';' || b == ':' || b == ','
}
