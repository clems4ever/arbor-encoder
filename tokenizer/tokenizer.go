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

const ArborOrderedAttribute = "arbor-ordered"

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

				stack = append(stack, &stackItem{childrenCounter: 0, ordered: isOrdered, pathIndex: myIndex})
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
