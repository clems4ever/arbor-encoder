package tokenizer

import (
	"os"
	"strings"
	"testing"
)

func TestTokenizer_UnregisteredAttributes(t *testing.T) {
	vocab := map[string]int{
		"<City>":        100,
		"</City>":       101,
		"@name":         102,
		"<__AttrPair>":  200,
		"</__AttrPair>": 201,
		"<__Key>":       202,
		"</__Key>":      203,
		"<__Value>":     204,
		"</__Value>":    205,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// xmlContent := `<City name="Paris" unknown="val"></City>`
	// Note: Attributes order is not guaranteed by xml parser usually?
	// Go's encoding/xml preserves attribute order?
	// Actually, `encoding/xml` does NOT guaranteed attribute order when unmarshalling into struct,
	// but here we use `decoder.Token()`. `decoder.Token()` parses attributes in the order they appear in the tag string?
	// Actually, the spec for XML says order of attributes is not significant.
	// `encoding/xml`'s `Attr` slide order depends on the parser implementation.
	// Let's assume for this specific string it is stable enough or we check content.

	xmlContent := `<City name="Paris" unknown="val"></City>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	tokens := res.Tokens
	paths := res.PaddedPaths

	// Helper to find a sequence of tokens
	findSequence := func(seq []int) int {
		for i := 0; i <= len(tokens)-len(seq); i++ {
			match := true
			for j := 0; j < len(seq); j++ {
				if tokens[i+j] != seq[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
		return -1
	}

	// Check for "unknown" structure
	// <__AttrPair>, <__Key>, ...

	// We expect the fallback structure to appear.
	// 200 (AttrPair), 202 (Key), ... "unknown" ..., 203 (KeyEnd), 204 (ValueStart), ... "val" ..., 205 (ValueEnd), 201 (AttrPairEnd)

	startUnknownIdx := findSequence([]int{200, 202})
	if startUnknownIdx == -1 {
		t.Fatalf("Did not find Start of Unknown Attribute structure (tokens 200, 202)")
	}

	// Verify paths at this location
	// The path for AttrPair (200) should be [0, 0] (since City is at [0], attr container is implicitly 0, so path is P_city + [0])
	// Wait, City is root.
	// Root path logic:
	// stack is empty. parentPath = []. myIndex = 0.
	// City path = [0].

	// Attributes logic:
	// nodePath = [0].
	// attrKeyPath = nodePath + [0] = [0, 0].

	// So AttrPair (200) should be at path [0, 0].
	// Path for Key (202) should be [0, 0, 0].

	if len(paths[startUnknownIdx]) < 2 {
		t.Errorf("Path too short for AttrPair")
	}
	// Deepest check

	// Check strict structure for "unknown" key
	// We don't know exactly what "unknown" tokenizes to with cl100k_base, but we can decode it back or just check path structure logic.

	// Let's assume "val" tokenizes to something.

	// Check that we have the sequence: KeyStart -> ... -> KeyEnd -> ValueStart

	keyEndIdx := findSequence([]int{203})
	if keyEndIdx == -1 {
		t.Errorf("Missing KeyEnd token")
	}

	valStartIdx := findSequence([]int{204})
	if valStartIdx == -1 {
		t.Errorf("Missing ValueStart token")
	}

	if keyEndIdx > valStartIdx {
		t.Errorf("KeyEnd appeared after ValueStart")
	}

	// Check paths for Value Content
	// They should be [0, 0, 1, i]
	// The ValueStart (204) is at [0, 0, 1]

	valStartPath := paths[valStartIdx]
	// Expected: [0, 0, 1] (padded: maybe [0, 0, 1, -1])

	// Only check prefix
	if len(valStartPath) < 3 || valStartPath[0] != 0 || valStartPath[1] != 0 || valStartPath[2] != 1 {
		t.Errorf("ValueStart path incorrect: %v. Expected prefix [0, 0, 1]", valStartPath)
	}
}
