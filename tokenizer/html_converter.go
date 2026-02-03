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
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			b.WriteString("<" + n.Data)
			for _, a := range n.Attr {
				// Simple escape for values
				val := strings.ReplaceAll(a.Val, "\"", "&quot;")
				if a.Key != "xmlns" { // avoid namespace issues if any
					b.WriteString(fmt.Sprintf(" %s=\"%s\"", a.Key, val))
				}
			}
			b.WriteString(">")
		case html.TextNode:
			data := strings.TrimSpace(n.Data)
			if data != "" {
				// Escape text
				data = strings.ReplaceAll(data, "&", "&amp;")
				data = strings.ReplaceAll(data, "<", "&lt;")
				data = strings.ReplaceAll(data, ">", "&gt;")
				b.WriteString(data)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}

		if n.Type == html.ElementNode {
			b.WriteString("</" + n.Data + ">")
		}
	}
	traverse(doc)
	return b.String(), nil
}
