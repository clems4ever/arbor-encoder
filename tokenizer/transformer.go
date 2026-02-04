package tokenizer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	// VirtualAttrTag is the XML tag used to wrap registered attributes in the virtual XML.
	VirtualAttrTag = "__Attr"
	// VirtualAttrName is the attribute name used to store the original attribute name in VirtualAttrTag.
	VirtualAttrName = "name"
)

type Transformer struct {
	vocab map[string]int
}

func NewTransformer(vocab map[string]int) *Transformer {
	return &Transformer{vocab: vocab}
}

// Transform converts standard XML into a valid XML stream where attributes are converted to child elements.
func (t *Transformer) Transform(r io.Reader) ([]byte, error) {
	var out bytes.Buffer
	// We don't use xml.Encoder because it's hard to control the exact output format (e.g. self-closing tags vs pairs)
	// and we want to ensure we don't mess up the vocabulary matching by introducing unwanted spaces or formatting.
	// However, simple xml.NewEncoder should be fine if we are careful.
	// Actually, manually constructing the tags gives us full control over <__Empty/> vs <__Empty></__Empty>.

	decoder := xml.NewDecoder(r)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			tagName := "<" + se.Name.Local + ">"
			if _, ok := t.vocab[tagName]; !ok {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}

			// Check for arbor-ordered attribute
			isOrdered := false
			for _, attr := range se.Attr {
				if attr.Name.Local == ArborOrderedAttribute {
					if attr.Value == "true" {
						isOrdered = true
					}
					break
				}
			}

			// Write Start Tag
			out.WriteString("<")
			out.WriteString(se.Name.Local)
			if isOrdered {
				out.WriteString(fmt.Sprintf(` %s="true"`, ArborOrderedAttribute))
			}
			out.WriteString(">")

			// Process Attributes
			for _, attr := range se.Attr {
				if attr.Name.Local == ArborOrderedAttribute {
					continue
				}
				if err := t.processAttribute(&out, attr); err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if _, ok := t.vocab[tagName]; !ok {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
			out.WriteString("</")
			out.WriteString(se.Name.Local)
			out.WriteString(">")

		case xml.CharData:
			content := string(se)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				// Escape content? The decoder unescapes it. We should re-escape.
				// Or assume content is safe? Better re-escape.
				buf := new(bytes.Buffer)
				if err := xml.EscapeText(buf, []byte(trimmed)); err != nil {
					return nil, err
				}
				out.Write(buf.Bytes())
			}
		}
	}

	return out.Bytes(), nil
}

func (t *Transformer) processAttribute(out *bytes.Buffer, attr xml.Attr) error {
	attrName := "@" + attr.Name.Local
	_, hasEmpty := t.vocab[TokenEmpty]
	// TokenValueEnd is implicit for registered attributes via the closing tag of __Attr,
	// but we must check if it exists in vocab during encoding. Transformer assumes it will be handled.

	if _, ok := t.vocab[attrName]; ok {
		// Registered Attribute
		// <__Attr name="foo">val</__Attr>
		out.WriteString(fmt.Sprintf(`<%s %s="%s">`, VirtualAttrTag, VirtualAttrName, attr.Name.Local))

		if attr.Value == "" && hasEmpty {
			// <__Empty/>
			out.WriteString(strings.ReplaceAll(TokenEmpty, "/", " /")) // Ensure valid XML self-closing if strictly needed, but <__Empty/> is fine.
		} else {
			if attr.Value != "" {
				buf := new(bytes.Buffer)
				if err := xml.EscapeText(buf, []byte(attr.Value)); err != nil {
					return err
				}
				out.Write(buf.Bytes())
			}
		}
		out.WriteString(fmt.Sprintf(`</%s>`, VirtualAttrTag))

	} else {
		// Unregistered Attribute
		// Expects special tokens
		var missing []string
		for _, tok := range []string{TokenAttrPair, TokenAttrPairEnd, TokenKey, TokenKeyEnd, TokenValue, TokenValueEnd} {
			if _, ok := t.vocab[tok]; !ok {
				missing = append(missing, tok)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("attribute %s not found in vocab, and special tokens (%s) are missing for fallback", attrName, strings.Join(missing, ", "))
		}

		// <__AttrPair>
		//   <__Key>name</__Key>
		//   <__Value>val</__Value>
		// </__AttrPair>
		
		// Note: We use the raw tokens strings from constants but stripped of < > because we construct XML.
		// TokenKey = "<__Key>" -> we write "<__Key>"
		
		// Helper to write element
		writeElem := func(tag string, val string) error {
			// tag is like "<__Key>"
			tagName := strings.Trim(tag, "<>")
			out.WriteString("<" + tagName + ">")
			buf := new(bytes.Buffer)
			if err := xml.EscapeText(buf, []byte(val)); err != nil {
				return err
			}
			out.Write(buf.Bytes())
			out.WriteString("</" + tagName + ">")
			return nil
		}

		// <__AttrPair>
		out.WriteString(TokenAttrPair)
		
		// Key
		if err := writeElem(TokenKey, attr.Name.Local); err != nil { return err }

		// Value
		out.WriteString(strings.TrimSuffix(TokenValue, ">") + ">") // <__Value>
		buf := new(bytes.Buffer)
		if err := xml.EscapeText(buf, []byte(attr.Value)); err != nil { return err }
		out.Write(buf.Bytes())
		out.WriteString(TokenValueEnd)

		out.WriteString(TokenAttrPairEnd)
	}
	return nil
}
