package tokenizer

import "strings"

// DecodeXML reconstructs the XML string from tokens, handling attributes and structure.
func (t *Tokenizer) DecodeXML(tokens []int) string {
	var sb strings.Builder

	tokenStr := func(id int) string {
		if tag, ok := t.vocabInv[id]; ok {
			return tag
		}
		return t.contentTokenizer.Decode([]int{id})
	}

	inTagHeader := false
	inAttribute := false
	attrPairState := 0 // 0: None, 1: Inside Pair, 2: Inside Key, 3: Inside Value

	for _, id := range tokens {
		s := tokenStr(id)

		// Special Unregistered Attribute Handling
		if s == TokenAttrPair {
			if inAttribute {
				sb.WriteString(`"`)
				inAttribute = false
			}
			attrPairState = 1
			continue
		}
		if s == TokenAttrPairEnd {
			attrPairState = 0
			continue
		}
		if s == TokenKey {
			attrPairState = 2
			sb.WriteString(" ") // Separator before attribute
			continue
		}
		if s == TokenKeyEnd {
			attrPairState = 1
			continue
		}
		if s == TokenValue {
			attrPairState = 3
			sb.WriteString(`="`)
			continue
		}
		if s == TokenValueEnd {
			if inAttribute {
				sb.WriteString(`"`)
				inAttribute = false
				continue
			}
			attrPairState = 1
			sb.WriteString(`"`)
			continue
		}

		if attrPairState == 2 {
			sb.WriteString(s)
			continue
		}
		if attrPairState == 3 {
			sb.WriteString(s)
			continue
		}

		if strings.HasPrefix(s, "@") {
			// Registered Attribute
			if inAttribute {
				sb.WriteString(`"`)
			}
			inAttribute = true
			sb.WriteString(" " + s[1:] + `="`)
		} else if strings.HasPrefix(s, "</") {
			// End Element
			if inAttribute {
				sb.WriteString(`"`)
				inAttribute = false
			}
			if inTagHeader {
				sb.WriteString(">")
				inTagHeader = false
			}
			sb.WriteString(s)

		} else if strings.HasPrefix(s, "<") {
			// Start Element
			if inAttribute {
				sb.WriteString(`"`)
				inAttribute = false
			}
			if inTagHeader {
				sb.WriteString(">")
			}
			tagName := strings.TrimSuffix(s, ">")
			sb.WriteString(tagName)
			inTagHeader = true
		} else {
			// Content
			if inAttribute {
				sb.WriteString(s)
			} else {
				if inTagHeader {
					// Assume content occurring in tag header means tag close
					sb.WriteString(">")
					inTagHeader = false
				}
				sb.WriteString(s)
			}
		}
	}

	if inAttribute {
		sb.WriteString(`"`)
	}
	if inTagHeader {
		sb.WriteString(">")
	}

	return sb.String()
}
