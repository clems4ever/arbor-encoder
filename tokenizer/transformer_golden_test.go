package tokenizer

import (
	"bytes"
	"encoding/xml"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func scanForVocab(r io.Reader) (map[string]int, error) {
	decoder := xml.NewDecoder(r)
	vocab := make(map[string]int)
	id := 1

	// Add special tags required by Transformer for fallback or delimiters
	special := []string{
		TokenAttrPair, TokenAttrPairEnd,
		TokenKey, TokenKeyEnd,
		TokenValue, TokenValueEnd,
		TokenEmpty,
	}
	for _, s := range special {
		vocab[s] = id
		id++
	}

	for {
		t, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch se := t.(type) {
		case xml.StartElement:
			tag := "<" + se.Name.Local + ">"
			if _, ok := vocab[tag]; !ok {
				vocab[tag] = id
				id++
			}
			for _, attr := range se.Attr {
				if attr.Name.Local == ArborOrderedAttribute {
					continue
				}
				attrName := "@" + attr.Name.Local
				if _, ok := vocab[attrName]; !ok {
					vocab[attrName] = id
					id++
				}
			}
		case xml.EndElement:
			tag := "</" + se.Name.Local + ">"
			if _, ok := vocab[tag]; !ok {
				vocab[tag] = id
				id++
			}
		}
	}
	return vocab, nil
}

// xmlNode represents a node in the XML tree for pretty printing
type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr
	Children []interface{} // *xmlNode or string
}

func parseXMLTree(r io.Reader) (*xmlNode, error) {
	dec := xml.NewDecoder(r)
	var stack []*xmlNode
	root := &xmlNode{XMLName: xml.Name{Local: "root"}} // Dummy root
	stack = append(stack, root)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			node := &xmlNode{XMLName: t.Name, Attrs: append([]xml.Attr(nil), t.Attr...)}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 1 { // Don't pop dummy root
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				str := string(t.Copy())
				// We intentionally don't TrimSpace here because we want to preserve significant whitespace,
				// but for pretty printing we might want to trim *insignificant* whitespace (between tags).
				// However, determining significance is hard.
				// For the "Virtual XML" which is machine generated, there is usually no whitespace between tags.
				// So any CharData is content.
				parent.Children = append(parent.Children, str)
			}
		}
	}
	return root, nil
}

func (n *xmlNode) prettyPrint(w io.Writer, depth int) {
	indent := strings.Repeat("  ", depth)

	// Determine if we should print inline (simple content) or block (complex content)
	isComplex := false
	for _, c := range n.Children {
		if _, ok := c.(*xmlNode); ok {
			isComplex = true
			break
		}
	}

	w.Write([]byte(indent))
	w.Write([]byte("<" + n.XMLName.Local))
	for _, attr := range n.Attrs {
		w.Write([]byte(" " + attr.Name.Local + `="` + attr.Value + `"`))
	}

	if len(n.Children) == 0 {
		w.Write([]byte(" />\n"))
		return
	}

	w.Write([]byte(">"))

	if isComplex {
		w.Write([]byte("\n"))
		for _, c := range n.Children {
			switch child := c.(type) {
			case *xmlNode:
				child.prettyPrint(w, depth+1)
			case string:
				// If mixed content exists in a complex node, we indent text too?
				// Or print as is.
				trimmed := strings.TrimSpace(child)
				if trimmed != "" {
					w.Write([]byte(strings.Repeat("  ", depth+1)))
					w.Write([]byte(trimmed)) // Might lose internal whitespace if we just print trimmed.
					w.Write([]byte("\n"))
				}
			}
		}
		w.Write([]byte(indent))
	} else {
		// All children are strings
		for _, c := range n.Children {
			if str, ok := c.(string); ok {
				// Escape Text
				xml.EscapeText(w, []byte(str))
			}
		}
	}

	w.Write([]byte("</" + n.XMLName.Local + ">\n"))
}

func indentXML(data []byte) ([]byte, error) {
	root, err := parseXMLTree(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	// Root children are the actual roots of the document
	for _, c := range root.Children {
		if node, ok := c.(*xmlNode); ok {
			node.prettyPrint(&buf, 0)
		}
	}
	return buf.Bytes(), nil
}

func TestTransformer_Golden(t *testing.T) {
	matches, err := filepath.Glob("testdata/*_golden.xml")
	if err != nil {
		t.Fatal(err)
	}

	for _, inFile := range matches {
		t.Run(filepath.Base(inFile), func(t *testing.T) {
			f, err := os.Open(inFile)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			// 1. Build Vocab from file to ensure we cover all tags/attributes
			// This simulates a "complete" vocab scenario.
			vocab, err := scanForVocab(f)
			if err != nil {
				t.Fatal(err)
			}
			f.Seek(0, 0) // Rewind

			// 2. Transform
			tr := NewTransformer(vocab)
			tokens, err := tr.Transform(f)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// 3. Serialize and Indent
			outBytes, err := indentXML(tokens)
			if err != nil {
				t.Fatalf("Indent failed: %v", err)
			}

			goldenFile := strings.TrimSuffix(inFile, ".xml") + "_virtual.xml"

			if *update {
				if err := os.WriteFile(goldenFile, outBytes, 0644); err != nil {
					t.Fatal(err)
				}
			}

			expected, err := os.ReadFile(goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file %s missing, run with -update to generate", goldenFile)
				}
				t.Fatal(err)
			}

			// Normalize line endings for comparison just in case
			if StringsDiff(string(expected), string(outBytes)) {
				t.Errorf("content mismatch for %s. Run with -update to fix.", goldenFile)
			}
		})
	}
}

func StringsDiff(a, b string) bool {
	// Simple equality for now
	return a != b
}
