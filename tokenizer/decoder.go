package tokenizer

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Element represents an XML node structure
type Element struct {
	Name       string
	Attributes []xml.Attr
	Children   []interface{} // *Element or string (CharData)
}

// String serializes the Element back to an XML string
func (e *Element) String() string {
	var sb strings.Builder
	sb.WriteString("<" + e.Name)
	for _, attr := range e.Attributes {
		sb.WriteString(" " + attr.Name.Local + `="` + attr.Value + `"`)
	}
	sb.WriteString(">")
	for _, child := range e.Children {
		switch c := child.(type) {
		case *Element:
			sb.WriteString(c.String())
		case string:
			sb.WriteString(c)
		}
	}
	sb.WriteString("</" + e.Name + ">")
	return sb.String()
}

// DecodeXML reconstructs the XML structure from tokens.
func (t *Tokenizer) DecodeXML(tokens []int) (*Element, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	// Helper to get string and vocab status
	getTokenInfo := func(id int) (string, bool) {
		if tag, ok := t.vocabInv[id]; ok {
			return tag, true
		}
		return t.contentTokenizer.Decode([]int{id}), false
	}

	var root *Element
	var stack []*Element

	i := 0
	for i < len(tokens) {
		id := tokens[i]
		s, isVocab := getTokenInfo(id)
		i++

		// Start Element (Must be in Vocab)
		if isVocab && strings.HasPrefix(s, "<") && !strings.HasPrefix(s, "</") &&
			s != TokenAttrPair && s != TokenKey && s != TokenValue &&
			s != TokenKeyEnd && s != TokenValueEnd && s != TokenAttrPairEnd {

			// Clean tag name
			tagName := strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">")
			el := &Element{Name: tagName}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, el)
			} else {
				root = el
			}
			stack = append(stack, el)
			continue
		}

		// End Element (Must be in Vocab)
		if isVocab && strings.HasPrefix(s, "</") && s != TokenAttrPairEnd && s != TokenKeyEnd && s != TokenValueEnd {
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected end tag: %s", s)
			}
			stack = stack[:len(stack)-1]
			continue
		}

		if len(stack) == 0 {
			// Ignore content outside root
			continue
		}

		current := stack[len(stack)-1]

		// Unregistered Attribute Sequence (checking token string constant)
		if isVocab && s == TokenAttrPair {
			var key, val strings.Builder
			state := 0 // 0: init, 1: key, 2: value

			// Consume loop
			for i < len(tokens) {
				subId := tokens[i]
				subS, subIsVocab := getTokenInfo(subId)
				i++

				if subIsVocab {
					if subS == TokenAttrPairEnd {
						break
					}
					if subS == TokenKey {
						state = 1
						continue
					}
					if subS == TokenKeyEnd {
						state = 0
						continue
					}
					if subS == TokenValue {
						state = 2
						continue
					}
					if subS == TokenValueEnd {
						state = 0
						continue
					}
				}

				switch state {
				case 1:
					key.WriteString(subS)
				case 2:
					val.WriteString(subS)
				}
			}
			current.Attributes = append(current.Attributes, xml.Attr{Name: xml.Name{Local: key.String()}, Value: val.String()})
			continue
		}

		// Registered Attribute (Must be in Vocab and start with @)
		if isVocab && strings.HasPrefix(s, "@") {
			attrName := s[1:]
			var valSb strings.Builder

			// Greedily consume value until TokenValueEnd or a tag
			for i < len(tokens) {
				// Lookahead
				if i >= len(tokens) {
					break
				}
				subId := tokens[i]
				subS, subIsVocab := getTokenInfo(subId)

				// Stop if delimiter (Must be Vocab)
				if subIsVocab && subS == TokenValueEnd {
					i++ // consume delimiter
					break
				}

				// Stop if start of new tag or end tag (fallback for missing delimiter)
				// Must be Vocab to be a structural stop
				if subIsVocab &&
					(strings.HasPrefix(subS, "<") || strings.HasPrefix(subS, "</")) &&
					subS != TokenAttrPair && subS != TokenKey && subS != TokenValue &&
					subS != TokenKeyEnd && subS != TokenValueEnd && subS != TokenAttrPairEnd {
					// We pushed back by NOT incrementing i
					break
				}
				// Stop if another attribute (Must be Vocab)
				if subIsVocab && strings.HasPrefix(subS, "@") {
					break
				}

				// Consume content
				i++
				valSb.WriteString(subS)
			}
			current.Attributes = append(current.Attributes, xml.Attr{Name: xml.Name{Local: attrName}, Value: valSb.String()})
			continue
		}

		// Skip special tokens if they appear out of place
		if isVocab && (s == TokenValueEnd || s == TokenAttrPairEnd || s == TokenKey || s == TokenKeyEnd || s == TokenValue) {
			continue
		}

		// Content
		// Merge with previous string if possible
		if len(current.Children) > 0 {
			if str, ok := current.Children[len(current.Children)-1].(string); ok {
				current.Children[len(current.Children)-1] = str + s
				continue
			}
		}
		current.Children = append(current.Children, s)
	}

	return root, nil
}
