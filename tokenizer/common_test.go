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
		"@extra":        base + 102,
		"<__AttrPair>":  base + 200,
		"</__AttrPair>": base + 201,
		"<__Key>":       base + 202,
		"</__Key>":      base + 203,
		"<__Value>":     base + 204,
		"</__Value>":    base + 205,
	}
	return createTempVocab(t, vocab)
}

func TestStructureLogic(t *testing.T) {
	vocabPath := createComprehensiveVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, _ := NewTokenizer(vocabPath)

	// Test 1: Sibling Indexing

	inputUnordered := `<Root><Child>A</Child><Child>B</Child></Root>`

	resU, _ := tokenizer.Tokenize(strings.NewReader(inputUnordered))

	var childIndices []int
	for i, tok := range resU.Tokens {
		// 200003 is <Child>
		if tok == 200003 {
			// Path structure for StartElement involves updating stack.
			// paths[i] is [0, 1] for Child A (Root is 0. Children start at 1).
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
