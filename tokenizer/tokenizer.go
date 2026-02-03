package tokenizer

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

const (
	ArborOrderedAttribute = "arbor-ordered"
	TokenAttrPair         = "<__AttrPair>"
	TokenAttrPairEnd      = "</__AttrPair>"
	TokenKey              = "<__Key>"
	TokenKeyEnd           = "</__Key>"
	TokenValue            = "<__Value>"
	TokenValueEnd         = "</__Value>"
)

type TokenizationResult struct {
	Tokens      []int
	PaddedPaths [][]int
}

type Tokenizer struct {
	vocab            map[string]int
	vocabInv         map[int]string
	contentTokenizer *tiktoken.Tiktoken
}

func NewTokenizer(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open vocab file: %w", err)
	}
	defer f.Close()

	var vocab map[string]int
	if err := json.NewDecoder(f).Decode(&vocab); err != nil {
		return nil, fmt.Errorf("failed to decode vocab file: %w", err)
	}

	vocabInv := make(map[int]string)
	for k, v := range vocab {
		vocabInv[v] = k
	}

	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	return &Tokenizer{
		vocab:            vocab,
		vocabInv:         vocabInv,
		contentTokenizer: tke,
	}, nil
}

func (t *Tokenizer) Tokenize(r io.Reader) (*TokenizationResult, error) {
	var tokens []int
	var paths [][]int

	// Stack to track the current path of indices.
	// Each element in the stack represents a level in the tree.
	// value: the current index at this level.
	// ordered: whether this level is an ordered collection.
	type stackItem struct {
		childrenCounter int // Counter for assigning indices to children
		ordered         bool
		pathIndex       int // The index of this node in its parent's scope (or 0 for root)
	}

	// Initialize stack with root level.
	// We assume the root level is ordered (sequences of root elements).
	// Root has no parent, so pathIndex is arbitrarily 0.
	stack := []*stackItem{}

	// depth tracks the nesting level (0-based)
	depth := 0

	decoder := xml.NewDecoder(r)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Helper to capture current path from the stack.
		// The path is defined by the sequence of `pathIndex` of all active nodes.
		getCurrentPath := func() []int {
			p := make([]int, len(stack))
			for i, item := range stack {
				p[i] = item.pathIndex
			}
			return p
		}

		switch se := token.(type) {
		case xml.StartElement:
			tagName := "<" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				isOrdered := false
				for _, attr := range se.Attr {
					if attr.Name.Local == ArborOrderedAttribute {
						if attr.Value == "true" {
							isOrdered = true
						}
						break
					}
				}

				var myIndex int
				var parentPath []int

				if len(stack) > 0 {
					parent := stack[len(stack)-1]

					myIndex = parent.childrenCounter
					parentPath = getCurrentPath()

					// Increment parent counter for the NEXT sibling, only if parent is ordered.
					if parent.ordered {
						parent.childrenCounter++
					}
				} else {
					myIndex = 0
					parentPath = []int{}
				}

				nodePath := make([]int, len(parentPath)+1)
				copy(nodePath, parentPath)
				nodePath[len(parentPath)] = myIndex

				tokens = append(tokens, id)
				paths = append(paths, nodePath)

				// Process Attributes
				// We behave as if all attributes are in a "virtual container" at index 0.
				for _, attr := range se.Attr {
					if attr.Name.Local == ArborOrderedAttribute {
						continue
					}
					if err := t.processAttribute(&tokens, &paths, attr, nodePath); err != nil {
						return nil, err
					}
				}

				// Push new stack item for children of this element
				// Children start at index 1 to reserve index 0 for attributes
				stack = append(stack, &stackItem{childrenCounter: 1, ordered: isOrdered, pathIndex: myIndex})
				depth++

			} else {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				depth--
				popped := stack[len(stack)-1]
				stack = stack[:len(stack)-1]

				// The End element belongs to the node we just closed.
				// So it should have the same path as the Start element.
				// We can reconstruct it from the popped item.

				// Reconstruct path: Current stack (parent) path + [popped.pathIndex]
				parentPath := getCurrentPath()
				nodePath := make([]int, len(parentPath)+1)
				copy(nodePath, parentPath)
				nodePath[len(parentPath)] = popped.pathIndex

				tokens = append(tokens, id)
				paths = append(paths, nodePath)

			} else {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.CharData:
			content := string(se)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				contentTokens := t.contentTokenizer.Encode(trimmed, nil, nil)
				parent := stack[len(stack)-1]

				for _, token := range contentTokens {
					tokens = append(tokens, token)

					// Path for content token:
					// Parent path (which describes the containing element) + [content_index]
					// Parent path is getCurrentPath().

					p := getCurrentPath()
					childPath := make([]int, len(p)+1)
					copy(childPath, p)
					childPath[len(p)] = parent.childrenCounter
					paths = append(paths, childPath)

					// Content tokens are always ordered sequentially.
					parent.childrenCounter++
				}
			}
		}
	}
	paddedPaths := getPaddedPaths(paths, 0, -1)
	return &TokenizationResult{
		Tokens:      tokens,
		PaddedPaths: paddedPaths,
	}, nil
}

// getPaddedPaths returns the paths as a 2D matrix.
// It pads shorter paths with padValue (usually -1).
func getPaddedPaths(paths [][]int, maxDepth int, padValue int) [][]int {
	// If maxDepth is 0, find the actual max depth in the data
	if maxDepth == 0 {
		for _, p := range paths {
			if len(p) > maxDepth {
				maxDepth = len(p)
			}
		}
	}

	tensor := make([][]int, len(paths))

	for i, p := range paths {
		row := make([]int, maxDepth)
		for j := 0; j < maxDepth; j++ {
			if j < len(p) {
				row[j] = p[j]
			} else {
				row[j] = padValue
			}
		}
		tensor[i] = row
	}

	return tensor
}

func (t *Tokenizer) Decode(tokens []int) string {
	var parts []string
	for _, token := range tokens {
		if tag, ok := t.vocabInv[token]; ok {
			parts = append(parts, tag)
		} else {
			val := t.contentTokenizer.Decode([]int{token})
			parts = append(parts, val)
		}
	}
	return strings.Join(parts, " ")
}

func (t *Tokenizer) processAttribute(tokens *[]int, paths *[][]int, attr xml.Attr, nodePath []int) error {
	attrName := "@" + attr.Name.Local

	// Pre-fetch special tokens mostly for Unregistered, but ValueEnd is used for Registered too now (for reversibility)
	// We only strictly need them if we use them.
	// Let's lazy fetch or check existence if we need them.

	valEndId, hasValEnd := t.vocab[TokenValueEnd]

	if attrId, ok := t.vocab[attrName]; ok {
		// Attribute Key Path: current node path + [0]
		attrKeyPath := make([]int, len(nodePath)+1)
		copy(attrKeyPath, nodePath)
		attrKeyPath[len(nodePath)] = 0

		*tokens = append(*tokens, attrId)
		*paths = append(*paths, attrKeyPath)

		// Attribute Value
		if attr.Value != "" {
			valTokens := t.contentTokenizer.Encode(attr.Value, nil, nil)
			for i, vt := range valTokens {
				*tokens = append(*tokens, vt)
				// Value Path: attrKeyPath + [i]
				valPath := make([]int, len(attrKeyPath)+1)
				copy(valPath, attrKeyPath)
				valPath[len(attrKeyPath)] = i
				*paths = append(*paths, valPath)
			}

			// DELIMITER for Registered Attributes
			// We append TokenValueEnd (</__Value>) to mark end of value.
			// This is necessary to distinguish AttrValue from subsequent CharData during decoding.
			if hasValEnd {
				*tokens = append(*tokens, valEndId)
				// Path for delimiter: same as attribute key level? or value level?
				// Logic: It terminates the value. It sits at the Key level structurally (sibling to value tokens? or parent?)
				// Unregistered uses: <__Value> (at key+1) ... content ... </__Value> (at key+1).
				// So let's put it at key+1 (valPath depth) but index?
				// Let's just use attrKeyPath (depth N+1).
				// Actually, reusing the path of the abstract container or the key seems safer.
				// In Unregistered: </__Value> is at `valNodePath` which is `keyNodePath` sibling? No.
				// structure: Pair -> ValueNode -> </Value>.
				// Here: Key -> ValueTokens -> EndToken.
				// Let's use attrKeyPath.
				*paths = append(*paths, attrKeyPath)
			}
		}
	} else {
		// Unregistered Attribute Handling
		// Check for required special tokens in vocab
		attrPairId, ok1 := t.vocab[TokenAttrPair]
		attrPairEndId, ok2 := t.vocab[TokenAttrPairEnd]
		keyId, ok3 := t.vocab[TokenKey]
		keyEndId, ok4 := t.vocab[TokenKeyEnd]
		valId, ok5 := t.vocab[TokenValue]
		// valEndId already fetched

		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !hasValEnd {
			return fmt.Errorf("attribute %s not found in vocab, and special tokens (%s, %s, %s, %s, %s, %s) are missing for fallback",
				attrName, TokenAttrPair, TokenAttrPairEnd, TokenKey, TokenKeyEnd, TokenValue, TokenValueEnd)
		}

		// 1. Emit <__AttrPair> at path + [0]
		attrPairPath := make([]int, len(nodePath)+1)
		copy(attrPairPath, nodePath)
		attrPairPath[len(nodePath)] = 0

		*tokens = append(*tokens, attrPairId)
		*paths = append(*paths, attrPairPath)

		// 2. Emit <__Key> at path + [0] + [0]
		keyNodePath := make([]int, len(attrPairPath)+1)
		copy(keyNodePath, attrPairPath)
		keyNodePath[len(attrPairPath)] = 0

		*tokens = append(*tokens, keyId)
		*paths = append(*paths, keyNodePath)

		// 3. Emit Key Content at path + [0] + [0] + [i]
		keyTokens := t.contentTokenizer.Encode(attr.Name.Local, nil, nil)
		for i, kt := range keyTokens {
			*tokens = append(*tokens, kt)
			contentPath := make([]int, len(keyNodePath)+1)
			copy(contentPath, keyNodePath)
			contentPath[len(keyNodePath)] = i
			*paths = append(*paths, contentPath)
		}

		// 4. Emit </__Key> at path + [0] + [0]
		*tokens = append(*tokens, keyEndId)
		*paths = append(*paths, keyNodePath)

		// 5. Emit <__Value> at path + [0] + [1]
		valNodePath := make([]int, len(attrPairPath)+1)
		copy(valNodePath, attrPairPath)
		valNodePath[len(attrPairPath)] = 1

		*tokens = append(*tokens, valId)
		*paths = append(*paths, valNodePath)

		// 6. Emit Value Content at path + [0] + [1] + [i]
		valTokens := t.contentTokenizer.Encode(attr.Value, nil, nil)
		for i, vt := range valTokens {
			*tokens = append(*tokens, vt)
			contentPath := make([]int, len(valNodePath)+1)
			copy(contentPath, valNodePath)
			contentPath[len(valNodePath)] = i
			*paths = append(*paths, contentPath)
		}

		// 7. Emit </__Value> at path + [0] + [1]
		*tokens = append(*tokens, valEndId)
		*paths = append(*paths, valNodePath)

		// 8. Emit </__AttrPair> at path + [0]
		*tokens = append(*tokens, attrPairEndId)
		*paths = append(*paths, attrPairPath)
	}
	return nil
}
