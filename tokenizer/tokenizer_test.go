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
	// Check paths roughly
	paths := res.Paths
	if len(paths[0]) != 2 {
		t.Errorf("Expected path length 2 for root, got %d", len(paths[0]))
	}
	if len(paths[1]) != 3 {
		t.Errorf("Expected path length 3 for child, got %d", len(paths[1]))
	}
	// Check last one
	if len(paths[len(paths)-1]) != 2 {
		t.Errorf("Expected path length 2 for root end, got %d", len(paths[len(paths)-1]))
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
	paths := res.Paths

	// Helper to find index of a structural token
	findIndex := func(target int) int {
		for i, tok := range tokens {
			if tok == target {
				return i
			}
		}
		return -1
	}

	// Verify structural path lengths (Length = Depth + 1)
	checks := []struct {
		tokenID     int
		expectedLen int
		name        string
	}{
		{200010, 2, "<Root>"},
		{200012, 3, "<Level1>"},
		{200014, 4, "<Level2>"},
		{200016, 5, "<Level3>"},
		{200017, 5, "</Level3>"},
		{200015, 4, "</Level2>"},
		{200013, 3, "</Level1>"},
		{200018, 3, "<Level1Sibling>"},
		{200019, 3, "</Level1Sibling>"},
		{200011, 2, "</Root>"},
	}

	for _, check := range checks {
		idx := findIndex(check.tokenID)
		if idx == -1 {
			t.Errorf("Token %s (%d) not found", check.name, check.tokenID)
			continue
		}
		if len(paths[idx]) != check.expectedLen {
			t.Errorf("Path length for %s mismatch: expected %d, got %d", check.name, check.expectedLen, len(paths[idx]))
		}
	}

	// Verify content depths
	// "Deep" should be at depth 4 (inside <Level3> which is at depth 3) -> Path length 5
	// We find the range between <Level3> and </Level3>
	startIdx := findIndex(200016) // <Level3>
	endIdx := findIndex(200017)   // </Level3>

	if startIdx != -1 && endIdx != -1 {
		if endIdx <= startIdx+1 {
			t.Error("Expected content tokens between <Level3> and </Level3>")
		}
		for i := startIdx + 1; i < endIdx; i++ {
			if len(paths[i]) != 6 {
				t.Errorf("Expected content path length for 'Deep' to be 6, got %d at index %d", len(paths[i]), i)
			}
		}
	}

	// "Shallow" should be at depth 2 (inside <Level1Sibling> which is at depth 1) -> Path length 3
	startIdx = findIndex(200018) // <Level1Sibling>
	endIdx = findIndex(200019)   // </Level1Sibling>

	if startIdx != -1 && endIdx != -1 {
		if endIdx <= startIdx+1 {
			t.Error("Expected content tokens between <Level1Sibling> and </Level1Sibling>")
		}
		for i := startIdx + 1; i < endIdx; i++ {
			if len(paths[i]) != 4 {
				t.Errorf("Expected content path length for 'Shallow' to be 4, got %d at index %d", len(paths[i]), i)
			}
		}
	}
}
