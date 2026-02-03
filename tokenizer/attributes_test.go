package tokenizer

import (
	"os"
	"strings"
	"testing"
)

func TestTokenizer_Attributes(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":    base + 100,
		"</City>":   base + 101,
		"<School>":  base + 102,
		"</School>": base + 103,
		"@name":     base + 104,
		"@zip":      base + 105,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	// City is ordered (default), has attributes.
	// Attributes should be at index 0.
	// School should be at index 1.
	xmlContent := `<City name="Paris" zip="75000"><School>S1</School></City>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	tokens := res.Tokens
	paths := res.PaddedPaths

	// Identify tokens
	// Tokens: City, @name, Paris*, @zip, 75000*, School, S1*, /School, /City
	// * = content tokens

	// 1. Check City
	if tokens[0] != base+100 {
		t.Errorf("Expected City token at 0")
	}
	// City path should be [0, -1, -1] assuming max depth of 3.
	if len(paths[0]) != 3 || paths[0][0] != 0 {
		t.Errorf("City path mismatch: %v", paths[0])
	}

	// 2. Check Attributes
	// Search for @name (104) and @zip (105)
	var zipIdx, nameIdx int
	for i, tok := range tokens {
		switch tok {
		case base + 104:
			nameIdx = i
		case base + 105:
			zipIdx = i
		}
	}

	if nameIdx == 0 || zipIdx == 0 {
		t.Fatal("Attribute tokens not found")
	}

	// 3. Check Attribute Values (content)
	// Value for name "Paris" is right after nameIdx
	parisIdx := nameIdx + 1
	// Path should be [0, 0, 0] (since it's the first token of value)
	if len(paths[parisIdx]) != 3 || paths[parisIdx][2] != 0 {
		t.Errorf("Paris value path invalid, expected ending in 0, got %v", paths[parisIdx])
	}

	// 4. Check School (Child Element)
	// Should be at index 1 (since 0 is reserved for attributes)
	schoolIdx := -1
	for i, tok := range tokens {
		if tok == base+102 {
			schoolIdx = i
			break
		}
	}

	if schoolIdx == -1 {
		t.Fatal("School token not found")
	}

	// Path should be [0, 1] (padded to [0, 1, -1])
	if paths[schoolIdx][1] != 1 {
		t.Errorf("School path invalid, expected [0, 1], got %v", paths[schoolIdx])
	}
}

func TestTokenizer_Attributes_UnorderedParent(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":  base + 100,
		"</City>": base + 101,
		"<Item>":  base + 102,
		"</Item>": base + 103,
		"@attr":   base + 104,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	// Arbor-ordered="false"
	// Attributes at 0.
	// Children at 1.
	xmlContent := `<City arbor-ordered="false" attr="val"><Item>A</Item><Item>B</Item></City>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	paths := res.PaddedPaths
	tokens := res.Tokens

	// Find Items
	var itemIndices []int
	for i, tok := range tokens {
		if tok == base+102 {
			itemIndices = append(itemIndices, i)
		}
	}

	if len(itemIndices) != 2 {
		t.Fatal("Expected 2 Item tokens")
	}

	// Both items should be at index 1 (unordered share index, and 0 is reserved)
	for _, idx := range itemIndices {
		p := paths[idx]
		// Expected: [0, 1]
		if p[1] != 1 {
			t.Errorf("Item path invalid for unordered container. Expected index 1, got path %v", p)
		}
	}

	// Find Attr
	attrIdx := -1
	for i, tok := range tokens {
		if tok == base+104 {
			attrIdx = i
			break
		}
	}
	// Expected: [0, 0]
	if attrIdx == -1 {
		t.Errorf("Attribute token not found")
	} else if paths[attrIdx][1] != 0 {
		t.Errorf("Attribute should be at index 0, got %v", paths[attrIdx])
	}
}

func TestTokenizer_Attributes_UnknownAttribute(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":  base + 100,
		"</City>": base + 101,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<City unknown="val"></City>`

	_, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err == nil {
		t.Error("Expected error for unknown attribute")
	} else if !strings.Contains(err.Error(), "attribute @unknown not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
