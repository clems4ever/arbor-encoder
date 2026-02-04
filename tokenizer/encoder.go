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
		childrenCounter int // Counter for assigning indices to children
		ordered         bool
		pathIndex       int // The index of this node in its parent's scope (or 0 for root)
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
				// Registered Attribute wrapper: <__Attr name="foo">
				// Get name
				var name string
				for _, attr := range se.Attr {
					if attr.Name.Local == VirtualAttrName {
						name = attr.Value
						break
					}
				}
				if name == "" {
					return nil, fmt.Errorf("missing name attribute in %s", VirtualAttrTag)
				}
				tagName = "@" + name
				isAttr = true
				isOrdered = true // Attributes content is ordered
			} else {
				// Standard tag or Special Tag (Unregistered group)
				tagName = "<" + se.Name.Local + ">"

				// Identify if it's a special tag that acts as attribute (index 0)
				if tagName == TokenAttrPair {
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
			stack = append(stack, &stackItem{childrenCounter: childrenStart, ordered: isOrdered, pathIndex: myIndex})

		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected end token </%s>, stack empty", se.Name.Local)
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
