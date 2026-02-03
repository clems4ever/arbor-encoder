package tokenizer

import (
	"os"
	"strings"
	"testing"
)

func createComprehensiveVocab(t *testing.T) string {
	base := 200000
	vocab := map[string]int{
		"<Root>":        base + 1,
		"</Root>":       base + 2,
		"<Child>":       base + 3,
		"</Child>":      base + 4,
		"<SubChild>":    base + 5,
		"</SubChild>":   base + 6,
		"<Leaf>":        base + 7,
		"</Leaf>":       base + 8,
		"@id":           base + 100,
		"@type":         base + 101,
		"@extra":        base + 102, // Added for Multi-depth with attributes test
		"<__AttrPair>":  base + 200,
		"</__AttrPair>": base + 201,
		"<__Key>":       base + 202,
		"</__Key>":      base + 203,
		"<__Value>":     base + 204,
		"</__Value>":    base + 205,
	}
	return createTempVocab(t, vocab)
}

func TestComprehensive(t *testing.T) {
	vocabPath := createComprehensiveVocab(t)
	defer os.Remove(vocabPath)

	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected []string // substrings expected in decoded output
		checkFn  func(t *testing.T, tokens []int, paths [][]int)
	}{
		{
			name: "Basic Ordered",
			input: `<Root arbor-ordered="true">
    <Child>A</Child>
    <Child>B</Child>
</Root>`,
			expected: []string{`<Root`, `<Child>A</Child>`, `<Child>B</Child>`, `</Root>`},
			checkFn: func(t *testing.T, tokens []int, paths [][]int) {
				// Check ordering logic in paths if possible.
				// Child A should have path sequence ending in X
				// Child B should have path sequence ending in X+1
				// Not easy to check from just flat arrays without manual parsing, but verify general logic?
			},
		},
		{
			name: "Unordered Children",
			input: `<Root arbor-ordered="false">
    <Child>A</Child>
    <Child>B</Child>
</Root>`,
			expected: []string{`<Root`, `<Child>A</Child>`, `<Child>B</Child>`, `</Root>`},
			checkFn: func(t *testing.T, tokens []int, paths [][]int) {
				// With arbor-ordered="false", sibling indices should stay same?
				// The logic in code:
				// if parent.ordered { parent.childrenCounter++ }
				// Default root is ordered usually but here we can't set root attribute easily unless we wrap it?
				// Actually the XML parser handles root attributes.
				// Code: "Initialize stack with root level. We assume the root level is ordered ... Root has no parent..."
				// The stack[0] is the root element context itself for its children?
				// When we see <Root>, we act as a child of the empty stack.
				// Then we push stackItem for <Root>'s children.
				// WE READ ATTRIBUTES of <Root>.
				// if "arbor-ordered" == "true" -> isOrdered = true.
				// Default looks like isOrdered = false in loop, unless found.
				// Wait.
				// In code:
				// isOrdered := false
				// for _, attr := range se.Attr { if "arbor-ordered" ... isOrdered=true }
				// stack = append(stack, &stackItem{... ordered: isOrdered ...})
				// So default is UNORDERED if attribute is missing?
				// Let's verify code.
				// "isOrdered := false"
				// So <Root> children are unordered by default unless arbor-ordered="true".
			},
		},
		{
			name:  "Unregistered Attributes",
			input: `<Root unknown="val" other="val2">Content</Root>`,
			expected: []string{ // DecodeXML adds spaces before attributes.
				`unknown="val"`, `other="val2"`, `Content`,
			},
		},
		{
			name:  "Mixed Registered and Unregistered Attributes",
			input: `<Root id="1" unknown="val" type="test">Content</Root>`,
			// Note: order of attributes in input string from test might be preserved or not by map iteration if we were generating it,
			// but here passing string directly.
			// Tokenizer iterates over xml.StartElement.Attr.
			expected: []string{
				`id="1"`, `unknown="val"`, `type="test"`,
			},
		},
		{
			name:  "Multi-depth",
			input: `<Root><Child><SubChild>Deep</SubChild></Child><Leaf>End</Leaf></Root>`,
			expected: []string{
				`<Root>`, `<Child>`, `<SubChild>Deep</SubChild>`, `</Child>`, `<Leaf>End</Leaf>`, `</Root>`,
			},
		},
		{
			name:  "Multi-depth with attributes",
			input: `<Root id="r1"><Child id="c1" extra="x"><SubChild>Deep</SubChild></Child></Root>`,
			expected: []string{
				`id="r1"`, `id="c1"`, `extra="x"`, `Deep`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			res, err := tokenizer.Tokenize(r)
			if err != nil {
				t.Fatalf("Tokenize failed: %v", err)
			}

			// Verify decoding reconstruction
			decoded := tokenizer.DecodeXML(res.Tokens)

			// Normalize spaces? DecodeXML output has specific spacing.
			// We check for substring containment of critical parts.
			for _, exp := range tt.expected {
				if !strings.Contains(decoded, exp) {
					t.Errorf("Expected output to contain '%s'. Got:\n%s", exp, decoded)
				}
			}

			if tt.checkFn != nil {
				tt.checkFn(t, res.Tokens, res.PaddedPaths)
			}
		})
	}
}

// Additional specific tests for Structure logic
func TestStructureLogic(t *testing.T) {
	vocabPath := createComprehensiveVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, _ := NewTokenizer(vocabPath)

	// Test 1: Sibling Indexing
	// Default is unordered (isOrdered := false in loop)
	// So distinct children should share index?
	// Wait, let's check code reading again.
	// "parent.childrenCounter++" is ONLY called "if parent.ordered".
	// So if unordered, counter stays same.

	inputUnordered := `<Root><Child>A</Child><Child>B</Child></Root>`
	// Root default unordered?
	// <Root> tag has no attributes -> isOrdered=false.
	// Stack for Root children -> ordered=false.
	// Child A: index = parent.childrenCounter (Start at 1). parent index not incremented.
	// Child B: index = parent.childrenCounter (Still 1).

	resU, _ := tokenizer.Tokenize(strings.NewReader(inputUnordered))
	// We need to identify tokens for Child A and Child B start.
	// Assuming <Child> is token base + 3 = 200003.

	var childIndices []int
	for i, tok := range resU.Tokens {
		// 200003 is <Child>
		if tok == 200003 {
			// Path structure for StartElement involves updating stack.
			// paths[i] is [0, 1] for Child A?
			// Root is 0. Children start at 1.
			// Let's check path of <Child> tokens.
			p := resU.PaddedPaths[i]
			// We expect path length >= 2. [RootIndex(0), ChildIndex]
			if len(p) >= 2 {
				childIndices = append(childIndices, p[1])
			}
		}
	}

	if len(childIndices) != 2 {
		t.Fatalf("Expected 2 Child tokens, got %d", len(childIndices))
	}
	if childIndices[0] != childIndices[1] {
		t.Errorf("Unordered siblings should share index. Got %d and %d", childIndices[0], childIndices[1])
	}

	// Test 2: Ordered Sibling Indexing
	inputOrdered := `<Root arbor-ordered="true"><Child>A</Child><Child>B</Child></Root>`
	resO, _ := tokenizer.Tokenize(strings.NewReader(inputOrdered))

	childIndices = []int{}
	for i, tok := range resO.Tokens {
		// Child is base + 3 = 200003
		if tok == 200003 {
			p := resO.PaddedPaths[i]
			if len(p) >= 2 {
				childIndices = append(childIndices, p[1])
			}
		}
	}

	if len(childIndices) != 2 {
		t.Fatalf("Expected 2 Child tokens in ordered test, got %d", len(childIndices))
	}
	if childIndices[0] == childIndices[1] {
		t.Errorf("Ordered siblings should increment index. Got %d and %d", childIndices[0], childIndices[1])
	}
}
