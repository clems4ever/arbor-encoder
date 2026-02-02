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
		"<Root>":   200010,
		"</Root>":  200011,
		"<Child>":  200012,
		"</Child>": 200013,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root>
		<Child>Value</Child>
	</Root>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	tokens := res.Tokens
	depths := res.Depths

	// We expect tokens for <Root>, <Child>, "Value", </Child>, </Root>
	// The token for "Value" depends on tiktoken encoding, so we won't hardcode it.
	// But we know there must be at least one token for "Value".
	if len(tokens) < 5 {
		t.Fatalf("Expected at least 5 tokens, got %d", len(tokens))
	}

	if tokens[0] != 200010 {
		t.Errorf("Expected first token to be 200010 (<Root>), got %d", tokens[0])
	}
	if tokens[1] != 200012 {
		t.Errorf("Expected second token to be 200012 (<Child>), got %d", tokens[1])
	}
	// content tokens in the middle
	if tokens[len(tokens)-2] != 200013 {
		t.Errorf("Expected second to last token to be 200013 (</Child>), got %d", tokens[len(tokens)-2])
	}
	if tokens[len(tokens)-1] != 200011 {
		t.Errorf("Expected last token to be 200011 (</Root>), got %d", tokens[len(tokens)-1])
	}

	// Depths:
	// <Root>: 0
	// <Child>: 1
	// "Value": 2 ...
	// </Child>: 1
	// </Root>: 0
	if depths[0] != 0 {
		t.Errorf("Expected first depth to be 0, got %d", depths[0])
	}
	if depths[1] != 1 {
		t.Errorf("Expected second depth to be 1, got %d", depths[1])
	}
	// Check content depths
	for i := 2; i < len(depths)-2; i++ {
		if depths[i] != 2 {
			t.Errorf("Expected content depth to be 2, got %d at index %d", depths[i], i)
		}
	}
	if depths[len(depths)-2] != 1 {
		t.Errorf("Expected second to last depth to be 1, got %d", depths[len(depths)-2])
	}
	if depths[len(depths)-1] != 0 {
		t.Errorf("Expected last depth to be 0, got %d", depths[len(depths)-1])
	}
}

func TestTokenizer_Tokenize_MissingTag(t *testing.T) {
	vocab := map[string]int{
		"<Root>":  200010,
		"</Root>": 200011,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root><Unknown>Val</Unknown></Root>`

	_, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
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
		"<Root>":  200010,
		"</Root>": 200011,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root><Unclosed></Root>`

	_, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err == nil {
		t.Error("Expected error for malformed XML, got nil")
	}
}

func TestTokenizer_Tokenize_Depth_DeepNesting(t *testing.T) {
	vocab := map[string]int{
		"<Root>":           200010,
		"</Root>":          200011,
		"<Level1>":         200012,
		"</Level1>":        200013,
		"<Level2>":         200014,
		"</Level2>":        200015,
		"<Level3>":         200016,
		"</Level3>":        200017,
		"<Level1Sibling>":  200018,
		"</Level1Sibling>": 200019,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root>
		<Level1>
			<Level2>
				<Level3>Deep</Level3>
			</Level2>
		</Level1>
		<Level1Sibling>Shallow</Level1Sibling>
	</Root>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	tokens := res.Tokens
	depths := res.Depths

	// Helper to find index of a structural token
	findIndex := func(target int) int {
		for i, tok := range tokens {
			if tok == target {
				return i
			}
		}
		return -1
	}

	// Verify structural depths
	checks := []struct {
		tokenID       int
		expectedDepth int
		name          string
	}{
		{200010, 0, "<Root>"},
		{200012, 1, "<Level1>"},
		{200014, 2, "<Level2>"},
		{200016, 3, "<Level3>"},
		{200017, 3, "</Level3>"},
		{200015, 2, "</Level2>"},
		{200013, 1, "</Level1>"},
		{200018, 1, "<Level1Sibling>"},
		{200019, 1, "</Level1Sibling>"},
		{200011, 0, "</Root>"},
	}

	for _, check := range checks {
		idx := findIndex(check.tokenID)
		if idx == -1 {
			t.Errorf("Token %s (%d) not found", check.name, check.tokenID)
			continue
		}
		if depths[idx] != check.expectedDepth {
			t.Errorf("Depth for %s mismatch: expected %d, got %d", check.name, check.expectedDepth, depths[idx])
		}
	}

	// Verify content depths
	// "Deep" should be at depth 4 (inside <Level3> which is at depth 3)
	// We find the range between <Level3> and </Level3>
	startIdx := findIndex(200016) // <Level3>
	endIdx := findIndex(200017)   // </Level3>

	if startIdx != -1 && endIdx != -1 {
		if endIdx <= startIdx+1 {
			t.Error("Expected content tokens between <Level3> and </Level3>")
		}
		for i := startIdx + 1; i < endIdx; i++ {
			if depths[i] != 4 {
				t.Errorf("Expected content depth for 'Deep' to be 4, got %d at index %d", depths[i], i)
			}
		}
	}

	// "Shallow" should be at depth 2 (inside <Level1Sibling> which is at depth 1)
	startIdx = findIndex(200018) // <Level1Sibling>
	endIdx = findIndex(200019)   // </Level1Sibling>

	if startIdx != -1 && endIdx != -1 {
		if endIdx <= startIdx+1 {
			t.Error("Expected content tokens between <Level1Sibling> and </Level1Sibling>")
		}
		for i := startIdx + 1; i < endIdx; i++ {
			if depths[i] != 2 {
				t.Errorf("Expected content depth for 'Shallow' to be 2, got %d at index %d", depths[i], i)
			}
		}
	}
}
