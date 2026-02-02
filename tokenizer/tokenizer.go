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

type TokenizationResult struct {
	Tokens []int
	Paths  [][]int
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
	stack := []*stackItem{{childrenCounter: 0, ordered: true, pathIndex: 0}}

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
				// Determine if this new element is ordered based on attribute
				isOrdered := true // Default to true
				for _, attr := range se.Attr {
					if attr.Name.Local == "ordered" {
						if attr.Value == "false" {
							isOrdered = false
						}
						break
					}
				}

				// The current stack top is the parent.
				parent := stack[len(stack)-1]

				// The index for this new node is the current value of the parent's counter.
				myIndex := parent.childrenCounter

				// Record the path for the Start Token.
				// The Start Token belongs to the node being created.
				// Its path should include the parent's path + [myIndex].
				// Wait, if we follow the recursive structure:
				// Root (path [0]) -> Child (path [0, 0])
				// But we are ABOUT to push the Child to stack.
				// The Start Token essentially *starts* the Child context.
				// So it should carry the path of the Child.

				// Construct path for this new node:
				// Parent path is getCurrentPath().
				// New path is parentPath + [myIndex].
				// But wait, our `getCurrentPath` iterates the stack.
				// We haven't pushed the new node yet.

				// Let's create the stack item first?
				// If we push first, getCurrentPath() will return [..., myIndex].
				// This seems cleaner.

				parentPath := getCurrentPath()
				nodePath := make([]int, len(parentPath)+1)
				copy(nodePath, parentPath)
				nodePath[len(parentPath)] = myIndex

				tokens = append(tokens, id)
				paths = append(paths, nodePath)

				// Push new stack item for children of this element
				stack = append(stack, &stackItem{childrenCounter: 0, ordered: isOrdered, pathIndex: myIndex})
				depth++

				// Increment parent counter for the NEXT sibling, only if parent is ordered.
				if parent.ordered {
					parent.childrenCounter++
				}

			} else {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				depth--
				// Pop the stack to close the current element context.
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

				// Content is children of the current stack top.
				parent := stack[len(stack)-1]

				for _, token := range contentTokens {
					tokens = append(tokens, token)

					// Path for content token:
					// Parent path (which describes the containing element) + [content_index]
					// Parent path is getCurrentPath().

					p := getCurrentPath()
					// Append the content's index (child of parent)
					childPath := make([]int, len(p)+1)
					copy(childPath, p)
					childPath[len(p)] = parent.childrenCounter
					paths = append(paths, childPath)

					// Content tokens are always ordered sequentially within the element?
					// Usually yes. Even if the element contains unordered *tags*,
					// the text content chunks are usually sequential.
					// We will increment the counter for each token.
					parent.childrenCounter++
				}
			}
		}
	}
	return &TokenizationResult{
		Tokens: tokens,
		Paths:  paths,
	}, nil
}

// GetPaddedPaths returns the paths as a flattened 1D array (row-major) with a stride equal to maxDepth.
// It pads shorter paths with padValue (usually -1 or 0, but be careful if 0 is a valid index).
// Since 0 is valid, we might want -1.
func (tr *TokenizationResult) GetPaddedPaths(maxDepth int, padValue int) ([]int, int) {
	// If maxDepth is 0, find the actual max depth in the data
	if maxDepth == 0 {
		for _, p := range tr.Paths {
			if len(p) > maxDepth {
				maxDepth = len(p)
			}
		}
	}

	tensor := make([]int, len(tr.Tokens)*maxDepth)

	// Initialize with padValue
	for i := range tensor {
		tensor[i] = padValue
	}

	for i, p := range tr.Paths {
		for j, val := range p {
			if j < maxDepth {
				tensor[i*maxDepth+j] = val
			}
		}
	}

	return tensor, maxDepth
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
