package tokenizer

import (
	"os"
	"strings"
	"testing"
)

func TestTokenizer_UnregisteredAttributes(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":        base + 100,
		"</City>":       base + 101,
		"@name":         base + 102,
		"<__UnregisteredAttr>":  base + 200,
		"</__UnregisteredAttr>": base + 201,
		"<__Key>":       base + 202,
		"</__Key>":      base + 203,
		"<__Value>":     base + 204,
		"</__Value>":    base + 205,
		"<__RegisteredAttr>":    base + 206,
		"</__RegisteredAttr>":   base + 207,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

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
	// base+200 (AttrPair), base+202 (Key), ... "unknown" ..., base+203 (KeyEnd), base+204 (ValueStart), ... "val" ..., base+205 (ValueEnd), base+201 (AttrPairEnd)

	startUnknownIdx := findSequence([]int{base + 200, base + 202})
	if startUnknownIdx == -1 {
		t.Fatalf("Did not find Start of Unknown Attribute structure (tokens %d, %d)", base+200, base+202)
	}

	// AttrPair (base+200) should be at path [0, 0].
	if len(paths[startUnknownIdx]) < 2 {
		t.Errorf("Path too short for AttrPair")
	}

	// Check strict structure for "unknown" key
	keyEndIdx := findSequence([]int{base + 203})
	if keyEndIdx == -1 {
		t.Errorf("Missing KeyEnd token")
	}

	valStartIdx := findSequence([]int{base + 204})
	if valStartIdx == -1 {
		t.Errorf("Missing ValueStart token")
	}

	if keyEndIdx > valStartIdx {
		t.Errorf("KeyEnd appeared after ValueStart")
	}

	// Check paths for Value Content
	// They should be [0, 0, 1, i]
	// The ValueStart (base+204) is at [0, 0, 1]

	valStartPath := paths[valStartIdx]

	// Only check prefix
	if len(valStartPath) < 3 || valStartPath[0] != 0 || valStartPath[1] != 0 || valStartPath[2] != 1 {
		t.Errorf("ValueStart path incorrect: %v. Expected prefix [0, 0, 1]", valStartPath)
	}
}
