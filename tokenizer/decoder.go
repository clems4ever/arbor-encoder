package tokenizer

import (
	"encoding/xml"
	"fmt"
	"strings"
)

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
			s != TokenUnregisteredAttr && s != TokenKey && s != TokenValue &&
			s != TokenKeyEnd && s != TokenValueEnd && s != TokenUnregisteredAttrEnd && s != TokenEmpty {

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
		if isVocab && strings.HasPrefix(s, "</") && s != TokenUnregisteredAttrEnd && s != TokenKeyEnd && s != TokenValueEnd {
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
		if isVocab && s == TokenUnregisteredAttr {
			var key, val strings.Builder
			state := 0 // 0: init, 1: key, 2: value

			// Consume loop
			for i < len(tokens) {
				subId := tokens[i]
				subS, subIsVocab := getTokenInfo(subId)
				i++

				if subIsVocab {
					if subS == TokenUnregisteredAttrEnd {
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

		// Registered Attribute (Must be in Vocab and start with ##)
		if isVocab && strings.HasPrefix(s, "##") {
			attrName := s[2:]
			var valSb strings.Builder

			// Check first token for explicit Empty value
			if i < len(tokens) {
				peekId := tokens[i]
				peekS, peekIsVocab := getTokenInfo(peekId)
				if peekIsVocab && peekS == TokenEmpty {
					// Explicit empty value
					i++ // consume <__Empty>
					current.Attributes = append(current.Attributes, xml.Attr{Name: xml.Name{Local: attrName}, Value: ""})
					continue
				}
			}

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
					subS != TokenUnregisteredAttr && subS != TokenKey && subS != TokenValue &&
					subS != TokenKeyEnd && subS != TokenValueEnd && subS != TokenUnregisteredAttrEnd {
					// We pushed back by NOT incrementing i
					break
				}
				// Stop if another attribute (Must be Vocab)
				if subIsVocab && strings.HasPrefix(subS, "##") {
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
		if isVocab && (s == TokenValueEnd || s == TokenUnregisteredAttrEnd || s == TokenKey || s == TokenKeyEnd || s == TokenValue || s == TokenEmpty) {
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
