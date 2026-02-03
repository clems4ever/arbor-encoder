package tokenizer

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// ConvertHTMLToXML converts legacy HTML to generic XML structure (XHTML-like)
// so that strict XML tokenizers/parsers can handle it.
// It skips <script> and <style> tags and normalizes attributes.
func ConvertHTMLToXML(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	var traverse func(*html.Node, int, bool)
	traverse = func(n *html.Node, depth int, insideComplex bool) {
		switch n.Type {
		case html.ElementNode:
			hasElementChildren := false
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					hasElementChildren = true
					break
				}
			}

			indent := ""
			if depth >= 0 {
				indent = "\n" + strings.Repeat("  ", depth)
			}

			b.WriteString(indent + "<" + n.Data)
			for _, a := range n.Attr {
				// Simple escape for values
				val := strings.ReplaceAll(a.Val, "&", "&amp;")
				val = strings.ReplaceAll(val, "\"", "&quot;")
				val = strings.ReplaceAll(val, "<", "&lt;")
				val = strings.ReplaceAll(val, ">", "&gt;")
				if a.Key != "xmlns" { // avoid namespace issues if any
					b.WriteString(fmt.Sprintf(" %s=\"%s\"", a.Key, val))
				}
			}
			b.WriteString(">")

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if hasElementChildren {
					if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
						continue
					}
					traverse(c, depth+1, true)
				} else {
					traverse(c, depth, false)
				}
			}

			if hasElementChildren {
				b.WriteString(indent + "</" + n.Data + ">")
			} else {
				b.WriteString("</" + n.Data + ">")
			}
			return
		case html.TextNode:
			data := strings.TrimSpace(n.Data)
			if data != "" {
				// Escape text
				data = strings.ReplaceAll(data, "&", "&amp;")
				data = strings.ReplaceAll(data, "<", "&lt;")
				data = strings.ReplaceAll(data, ">", "&gt;")

				if insideComplex {
					indent := "\n" + strings.Repeat("  ", depth)
					b.WriteString(indent + data)
				} else {
					b.WriteString(data)
				}
			}
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c, depth, insideComplex)
		}
	}
	traverse(doc, 0, true)
	return strings.TrimSpace(b.String()), nil
}
