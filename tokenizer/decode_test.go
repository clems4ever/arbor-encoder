package tokenizer

import (
	"os"
	"strings"
	"testing"
)

func TestTokenizer_DecodeXML(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":        base + 10,
		"</City>":       base + 11,
		"<School>":      base + 12,
		"</School>":     base + 13,
		"@name":         base + 14,
		"<__AttrPair>":  base + 200,
		"</__AttrPair>": base + 201,
		"<__Key>":       base + 202,
		"</__Key>":      base + 203,
		"<__Value>":     base + 204,
		"</__Value>":    base + 205,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	// Case 1: Simple attributes
	input1 := `<City name="Paris">Hello</City>`
	res1, err := tokenizer.Tokenize(strings.NewReader(input1))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	decodedStruct1, err := tokenizer.DecodeXML(res1.Tokens)
	if err != nil {
		t.Fatalf("DecodeXML failed: %v", err)
	}
	decoded1 := decodedStruct1.String()
	// Note: DecodeXML might produce spacing differences depending on tokenizer behavior for "Hello" (space prefix?)
	// But structure should be correct.
	if !strings.Contains(decoded1, `<City name="Paris">`) {
		t.Errorf("DecodeXML Case 1 failed. Got: %s", decoded1)
	}
	if !strings.Contains(decoded1, `Hello</City>`) {
		t.Errorf("DecodeXML Case 1 content failed. Got: %s", decoded1)
	}

	// Case 2: Unregistered attributes
	// <City zip="75000">
	input2 := `<City zip="75000"></City>`
	res2, err := tokenizer.Tokenize(strings.NewReader(input2))
	if err != nil {
		t.Fatalf("Tokenize failed for 2: %v", err)
	}
	decodedStruct2, err := tokenizer.DecodeXML(res2.Tokens)
	if err != nil {
		t.Fatalf("DecodeXML failed: %v", err)
	}
	decoded2 := decodedStruct2.String()
	if !strings.Contains(decoded2, `zip="75000"`) {
		t.Errorf("DecodeXML Case 2 failed. Got: %s", decoded2)
	}

	// Case 3: Mixed
	input3 := `<City name="Paris" zip="75000"><School>S1</School></City>`
	res3, _ := tokenizer.Tokenize(strings.NewReader(input3))
	decodedStruct3, err := tokenizer.DecodeXML(res3.Tokens)
	if err != nil {
		t.Fatalf("DecodeXML failed: %v", err)
	}
	decoded3 := decodedStruct3.String()

	// We want to check that it is valid XML roughly
	expectedParts := []string{`<City`, `name="Paris"`, `zip="75000"`, `><School`, `>S1</School>`, `</City>`}
	for _, part := range expectedParts {
		if !strings.Contains(decoded3, part) {
			t.Errorf("DecodeXML Case 3 missing part '%s'. Got: %s", part, decoded3)
		}
	}
}
