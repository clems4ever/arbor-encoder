package tokenizer

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

type Encoder struct {
	vocab            map[string]int
	contentTokenizer *tiktoken.Tiktoken
}

func NewEncoder(vocab map[string]int, contentTokenizer *tiktoken.Tiktoken) *Encoder {
	return &Encoder{
		vocab:            vocab,
		contentTokenizer: contentTokenizer,
	}
}

func (e *Encoder) Encode(r io.Reader) (*TokenizationResult, error) {
	var tokens []int
	var paths [][]int

	type stackItem struct {
		childrenCounter  int // Counter for assigning indices to children
		ordered          bool
		pathIndex        int // The index of this node in its parent's scope (or 0 for root)
		isRegisteredAttr bool
	}

	// We assume a virtual root if we really wanted, but here we just start processing.
	// But Wait! The `paths` usually start at depth 0 or 1?
	// In previous logic (Phase 1+2 combined): stack [] meant root level.
	// `tokens` logic appended root elements.
	// Let's replicate this.
	stack := []*stackItem{}

	// Helper to capture current path from the stack.
	getCurrentPath := func() []int {
		p := make([]int, len(stack))
		for i, item := range stack {
			p[i] = item.pathIndex
		}
		return p
	}

	// extractRegisteredAttrName reads the <__Key>...</__Key><__Value> sequence
	// and returns the attribute name. It consumes the open tag of <__Value>.
	extractRegisteredAttrName := func(dec *xml.Decoder) (string, error) {
		// 1. Expect <__Key>
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != strings.Trim(TokenKey, "<>") {
			return "", fmt.Errorf("expected %s after %s, got %v", TokenKey, VirtualAttrTag, tok)
		}

		// 2. Expect Name (CharData)
		tok, err = dec.Token()
		if err != nil {
			return "", err
		}
		cd, ok := tok.(xml.CharData)
		if !ok {
			return "", fmt.Errorf("expected CharData in %s", TokenKey)
		}
		name := string(cd)

		// 3. Expect </__Key>
		tok, err = dec.Token()
		if err != nil {
			return "", err
		}
		ee, ok := tok.(xml.EndElement)
		if !ok || ee.Name.Local != strings.Trim(TokenKey, "<>") {
			// Handle case where CharData might be followed by EndElement directly
			// But check if we missed EndElement above?
			// The loop above consumed CharData. Next must be EndElement.
			return "", fmt.Errorf("expected %s, got %v", TokenKeyEnd, tok)
		}

		// 4. Expect <__Value>
		tok, err = dec.Token()
		if err != nil {
			return "", err
		}
		seVal, ok := tok.(xml.StartElement)
		if !ok || seVal.Name.Local != strings.Trim(TokenValue, "<>") {
			return "", fmt.Errorf("expected %s start, got %v", TokenValue, tok)
		}

		return name, nil
	}

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
			var tagName string
			isOrdered := false
			isAttr := false

			// Handle different tag types
			if se.Name.Local == VirtualAttrTag {
				// Registered Attribute wrapper: <__Attr>...
				// Expect <__Key>name</__Key><__Value>
				name, err := extractRegisteredAttrName(decoder)
				if err != nil {
					return nil, err
				}

				tagName = "@" + name
				isAttr = true
				isOrdered = true // Attributes content is ordered
			} else {
				// Standard tag or Special Tag (Unregistered group)
				tagName = "<" + se.Name.Local + ">"

				// Identify if it's a special tag that acts as attribute (index 0)
				// Note: <__Key> is consumed inside extractRegisteredAttrName ONLY if inside __RegisteredAttr.
				// If we see it here, it must be inside __UnregisteredAttr (Unregistered).
				if tagName == TokenUnregisteredAttr {
					isAttr = true
					isOrdered = true
				}
				// Note: <__Key> and <__Value> are children of <__AttrPair>.
				// They should follow standard indexing (0 then 1) if AttrPair is ordered.

				// Handle <__Empty> which maps to <__Empty/> in vocab
				if tagName == "<__Empty>" {
					tagName = TokenEmpty
				}

				// Note: <__Value> is not IsAttr (index 1), handled by default count=1 logic unless count was reset
				if tagName == TokenValue {
					isOrdered = true
				}

				// Check arbor-ordered
				for _, attr := range se.Attr {
					if attr.Name.Local == ArborOrderedAttribute {
						if attr.Value == "true" {
							isOrdered = true
						}
						break
					}
				}
			}

			// Vocab Lookup
			id, ok := e.vocab[tagName]
			if !ok {
				// Fallback for <__Value> if we are inside Unregistered
				// Actually <__Value> is in vocab.
				return nil, fmt.Errorf("token %s not found in vocab", tagName)
			}

			// Path Logic
			var myIndex int
			var parentPath []int

			if len(stack) > 0 {
				parent := stack[len(stack)-1]

				// Index Logic
				// If we are starting a node that "belongs" to attribute bucket (index 0)
				if isAttr {
					myIndex = 0
				} else {
					myIndex = parent.childrenCounter
					if parent.ordered {
						parent.childrenCounter++
					}
				}
				parentPath = getCurrentPath()
			} else {
				myIndex = 0
				parentPath = []int{}
			}

			// Build Path
			nodePath := make([]int, len(parentPath)+1)
			copy(nodePath, parentPath)
			nodePath[len(parentPath)] = myIndex

			tokens = append(tokens, id)
			paths = append(paths, nodePath)

			// Push Stack
			childrenStart := 1
			// Compatibility: Registered attributes start content at index 0.
			// Special nodes (AttrPair, Key, Value) also start content at index 0.
			if isAttr || strings.HasPrefix(tagName, "<__") {
				childrenStart = 0
			}
			stack = append(stack, &stackItem{
				childrenCounter:  childrenStart,
				ordered:          isOrdered,
				pathIndex:        myIndex,
				isRegisteredAttr: se.Name.Local == VirtualAttrTag,
			})

		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected end token </%s>, stack empty", se.Name.Local)
			}

			// Ignore closing tag of __Value if inside registered attribute
			if se.Name.Local == strings.Trim(TokenValue, "<>") && stack[len(stack)-1].isRegisteredAttr {
				continue
			}

			popped := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			var tagName string
			if se.Name.Local == VirtualAttrTag {
				// End of <__Attr> -> Emit </__Value>
				tagName = TokenValueEnd
			} else {
				tagName = "</" + se.Name.Local + ">"
			}

			id, ok := e.vocab[tagName]
			if ok {
				// Path logic
				parentPath := getCurrentPath() // Now pointing to parent of popped
				nodePath := make([]int, len(parentPath)+1)
				copy(nodePath, parentPath)
				nodePath[len(parentPath)] = popped.pathIndex

				tokens = append(tokens, id)
				paths = append(paths, nodePath)
			}
			// If not in vocab (phantom), ignore. <__Empty/> handling often means no End token.

		case xml.CharData:
			content := string(se)
			// Trimming whitespace is risky because we might lose significant space in attribute values
			// (e.g. class="foo "). The Transformer already handles trimming of original structural text.
			// So any CharData we see here is likely significant.

			if len(content) == 0 {
				continue
			}

			if len(stack) == 0 {
				continue
			}
			parent := stack[len(stack)-1]

			contentTokens := e.contentTokenizer.Encode(content, nil, nil)
			for _, t := range contentTokens {
				tokens = append(tokens, t)

				// Path logic for content
				p := getCurrentPath()
				childPath := make([]int, len(p)+1)
				copy(childPath, p)
				childPath[len(p)] = parent.childrenCounter
				paths = append(paths, childPath)

				// Content is always ordered
				parent.childrenCounter++
			}
		}
	}

	paddedPaths := getPaddedPaths(paths, 0, -1)
	return &TokenizationResult{
		Tokens:      tokens,
		PaddedPaths: paddedPaths,
	}, nil
}
