package tokenizer

import (
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

// Transform converts standard XML into a valid XML object where attributes are converted to child elements.
func (t *Transformer) Transform(r io.Reader) (*Element, error) {
	decoder := xml.NewDecoder(r)
	var stack []*Element
	var root *Element

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

			el := &Element{Name: se.Name.Local}

			// Check for arbor-ordered attribute
			for _, attr := range se.Attr {
				if attr.Name.Local == ArborOrderedAttribute {
					if attr.Value == "true" {
						el.Attributes = append(el.Attributes, attr)
					}
					break
				}
			}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, el)
			} else {
				root = el
			}
			stack = append(stack, el)

			// Process Attributes
			for _, attr := range se.Attr {
				if attr.Name.Local == ArborOrderedAttribute {
					continue
				}
				if err := t.processAttributeToElement(el, attr); err != nil {
					return nil, err
				}
			}

		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if _, ok := t.vocab[tagName]; !ok {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected end element %s", se.Name.Local)
			}
			stack = stack[:len(stack)-1]

		case xml.CharData:
			content := string(se)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				if len(stack) > 0 {
					current := stack[len(stack)-1]
					current.Children = append(current.Children, trimmed)
				}
			}
		}
	}

	return root, nil
}

func (t *Transformer) processAttributeToElement(parent *Element, attr xml.Attr) error {
	attrName := "@" + attr.Name.Local
	_, hasEmpty := t.vocab[TokenEmpty]

	if _, ok := t.vocab[attrName]; ok {
		// Registered Attribute
		child := &Element{
			Name:       VirtualAttrTag,
			Attributes: []xml.Attr{{Name: xml.Name{Local: VirtualAttrName}, Value: attr.Name.Local}},
		}

		if attr.Value == "" && hasEmpty {
			// <__Empty/>
			// Represent as Element with Name "__Empty" and no child.
			emptyName := strings.Trim(TokenEmpty, "<> /") // Strip < > /
			child.Children = append(child.Children, &Element{Name: emptyName})
		} else {
			if attr.Value != "" {
				child.Children = append(child.Children, attr.Value)
			}
		}
		parent.Children = append(parent.Children, child)

	} else {
		// Unregistered Attribute
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
		pairName := strings.Trim(TokenAttrPair, "<>")
		pair := &Element{Name: pairName}

		// <__Key>name</__Key>
		keyName := strings.Trim(TokenKey, "<>")
		pair.Children = append(pair.Children, &Element{
			Name:     keyName,
			Children: []interface{}{attr.Name.Local},
		})

		// <__Value>val</__Value>
		valName := strings.Trim(TokenValue, "<>")
		pair.Children = append(pair.Children, &Element{
			Name:     valName,
			Children: []interface{}{attr.Value},
		})

		parent.Children = append(parent.Children, pair)
	}
	return nil
}
