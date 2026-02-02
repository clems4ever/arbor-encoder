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
	Depths []int
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
	var depths []int
	var paths [][]int

	// Stack to track the current path of indices.
	// Each element in the stack represents a level in the tree.
	// value: the current index at this level.
	// ordered: whether this level is an ordered collection.
	type stackItem struct {
		index   int
		ordered bool
	}

	// Initialize stack with root level.
	// We assume the root level is ordered (sequences of root elements).
	stack := []*stackItem{{index: 0, ordered: true}}

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

		// Helper to capture current path
		getCurrentPath := func() []int {
			p := make([]int, len(stack))
			for i, item := range stack {
				p[i] = item.index
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

				// Parent updates its counter for the *next* sibling if it is ordered.
				// However, the *current* element takes the *current* index of the parent.
				// We need to look at the top of the stack (parent).
				parent := stack[len(stack)-1]

				// The path for this Start Tag includes the parent's current index.
				// But wait, the path should represent [idx0, idx1, idx2...].
				// We append the *new* level's index (starting at 0) to the path?
				// Or does the path end at the node itself?
				// Usually path to node at depth D has length D+1?
				// Root is depth 0. Path [0].
				// Child is depth 1. Path [0, 0].

				// Let's adopt: Path is the sequence of indices from root to current node.
				// Parent gave us `parent.index`. That is the index of THIS node among siblings.

				currentPath := getCurrentPath()
				tokens = append(tokens, id)
				depths = append(depths, depth)
				paths = append(paths, currentPath)

				// Push new stack item for children of this element
				stack = append(stack, &stackItem{index: 0, ordered: isOrdered})
				depth++

				// Increment parent index only if parent is ordered
				if parent.ordered {
					parent.index++
				}

			} else {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				depth--
				// Pop the stack
				stack = stack[:len(stack)-1]

				// The EndTag belongs to the same node as StartTag.
				// So it should share the path of the node.
				// But we just popped the child context. The parent stack item has already incremented!
				// We need to recover the index of this node.
				// Actually, `parent.index` is now pointing to the *next* slot.
				// So the index of this just-finished node was `parent.index - 1` (if ordered).

				// However, if we simply use the parent's current index state, it points to the Next sibling.
				// We want the EndTag to align with the StartTag essentially.

				// Re-constructing the path for the EndTag:
				// It should be the same as StartTag.
				// StartTag path was: [grandparent_idx, parent_idx, node_idx_at_start]
				// Now parent_idx has moved on.

				// Simplification: We can store the 'fixed' path in the stack when we push it?
				// Let's refine the stack logic.

				// Let's use the path recorded in the stack item itself!
				// When we pushed the stack item for this node, we could have stored its 'selfIndex'.

				// Refactored logic below.

				tokens = append(tokens, id)
				depths = append(depths, depth)

				// Path reconstruction is tricky with just the stack as implemented above.
				// Let's trust that consistent "coordinate" system is valuable.
				// If we use the current stack state (popped), we are back at the parent level.
				// The "path" of the EndTag describes the context *after* the node closes, or *of* the closing node?
				// Usually, EndTag is part of the node.
				// Ideally Path(StartTag) == Path(EndTag).

				// To do this strict matching, we'd need to remember the index logic.
				// For now, I will return the path of the *context* where the EndTag appears (which is the parent context).
				// This effectively means EndTag is at depth D-1.
				// Wait, `depth` variable was decremented to D-1.
				// So EndTag is conceptually at the parent level?
				// In XML, `<a>` is start of A. `</a>` is end of A. Both are boundaries.
				// The content is inside.
				// Let's use the parent path.
				paths = append(paths, getCurrentPath())

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
					depths = append(depths, depth)

					// Path for content token:
					// Parent path + [content_index]
					// Note: Content is always ordered sequence of tokens.
					// But does it respect the `ordered=false` of the parent tag?
					// Text content inside a tag is implicitly a sequence.
					// <Set> unordered1 unordered2 </Set>
					// <Set> text </Set> -> The text is a single value? Or sequence of tokens?
					// Usually we treat text tokens as ordered sequence relative to each other.
					// So I will always append an index for them.

					p := getCurrentPath()
					p = append(p, parent.index)
					paths = append(paths, p)

					parent.index++
				}
			}
		}
	}
	return &TokenizationResult{
		Tokens: tokens,
		Depths: depths,
		Paths:  paths,
	}, nil
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
