package tokenizer

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func createTempVocab(t *testing.T, vocab map[string]int) string {
	tmpFile, err := os.CreateTemp("", "vocab-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if err := json.NewEncoder(tmpFile).Encode(vocab); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	return tmpFile.Name()
}

func TestNewTokenizer_Success(t *testing.T) {
	vocab := map[string]int{
		"<Test>":  1,
		"</Test>": 2,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("NewTokenizer failed: %v", err)
	}

	if tokenizer == nil {
		t.Fatal("Expected tokenizer to be non-nil")
	}
	if len(tokenizer.vocab) != 2 {
		t.Errorf("Expected vocab size 2, got %d", len(tokenizer.vocab))
	}
}

func TestNewTokenizer_FileNotFound(t *testing.T) {
	_, err := NewTokenizer("non-existent-file.json")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestTokenizer_Tokenize_Success(t *testing.T) {
	vocab := map[string]int{
		"<Root>":   10,
		"</Root>":  11,
		"<Child>":  12,
		"</Child>": 13,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root>
		<Child>Value</Child>
	</Root>`

	tokens, depths, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expectedTokens := []int{10, 12, 13, 11}
	expectedDepths := []int{0, 1, 1, 0}

	if len(tokens) != len(expectedTokens) {
		t.Fatalf("Expected %d tokens, got %d", len(expectedTokens), len(tokens))
	}

	for i, token := range tokens {
		if token != expectedTokens[i] {
			t.Errorf("Token at index %d mismatch: expected %d, got %d", i, expectedTokens[i], token)
		}
	}

	if len(depths) != len(expectedDepths) {
		t.Fatalf("Expected %d depths, got %d", len(expectedDepths), len(depths))
	}

	for i, depth := range depths {
		if depth != expectedDepths[i] {
			t.Errorf("Depth at index %d mismatch: expected %d, got %d", i, expectedDepths[i], depth)
		}
	}
}

func TestTokenizer_Tokenize_MissingTag(t *testing.T) {
	vocab := map[string]int{
		"<Root>":  10,
		"</Root>": 11,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root><Unknown>Val</Unknown></Root>`

	_, _, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err == nil {
		t.Error("Expected error for unknown tag, got nil")
	}

	expectedErrorFragment := "tag <Unknown> not found in vocab"
	if !strings.Contains(err.Error(), expectedErrorFragment) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedErrorFragment, err.Error())
	}
}

func TestTokenizer_Tokenize_MalformedXML(t *testing.T) {
	vocab := map[string]int{
		"<Root>":  10,
		"</Root>": 11,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root><Unclosed></Root>`

	_, _, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err == nil {
		t.Error("Expected error for malformed XML, got nil")
	}
}
