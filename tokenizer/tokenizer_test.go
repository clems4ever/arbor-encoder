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

func countValid(p []int) int {
	c := 0
	for _, v := range p {
		if v != -1 {
			c++
		}
	}
	return c
}

func TestNewTokenizer_Success(t *testing.T) {
	vocab := map[string]int{
		"<Test>":  200001,
		"</Test>": 200002,
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
	paths := res.PaddedPaths
	// Due to path fix, Root path is now [0], not [0, 0]
	// Root depth 0 -> Path len 1
	if countValid(paths[0]) != 1 {
		t.Errorf("Expected path length 1 for root, got %d", countValid(paths[0]))
	}
	// Child depth 1 -> Path len 2
	if countValid(paths[1]) != 2 {
		t.Errorf("Expected path length 2 for child, got %d", countValid(paths[1]))
	}
	// Check last one
	if countValid(paths[len(paths)-1]) != 1 {
		t.Errorf("Expected path length 1 for root end, got %d", countValid(paths[len(paths)-1]))
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
	paths := res.PaddedPaths

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
		expectedLen int // Now expectedLen = depth + 1 (Root starts at [0], len 1)
		name        string
	}{
		{200010, 1, "<Root>"},
		{200012, 2, "<Level1>"},
		{200014, 3, "<Level2>"},
		{200016, 4, "<Level3>"},
		{200017, 4, "</Level3>"},
		{200015, 3, "</Level2>"},
		{200013, 2, "</Level1>"},
		{200018, 2, "<Level1Sibling>"},
		{200019, 2, "</Level1Sibling>"},
		{200011, 1, "</Root>"},
	}

	for _, check := range checks {
		idx := findIndex(check.tokenID)
		if idx == -1 {
			t.Errorf("Token %s (%d) not found", check.name, check.tokenID)
			continue
		}
		if countValid(paths[idx]) != check.expectedLen {
			t.Errorf("Path length for %s mismatch: expected %d, got %d", check.name, check.expectedLen, countValid(paths[idx]))
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
			if countValid(paths[i]) != 5 {
				t.Errorf("Expected content path length for 'Deep' to be 5, got %d at index %d", countValid(paths[i]), i)
			}
		}
	}

	// "Shallow" should be at depth 2 (inside <Level1Sibling> which is at depth 1) -> Path length 3 (root + level1sibling + childIdx)
	startIdx = findIndex(200018) // <Level1Sibling>
	endIdx = findIndex(200019)   // </Level1Sibling>

	if startIdx != -1 && endIdx != -1 {
		if endIdx <= startIdx+1 {
			t.Error("Expected content tokens between <Level1Sibling> and </Level1Sibling>")
		}
		for i := startIdx + 1; i < endIdx; i++ {
			if countValid(paths[i]) != 3 {
				t.Errorf("Expected content path length for 'Shallow' to be 3, got %d at index %d", countValid(paths[i]), i)
			}
		}
	}
}

func TestTokenizer_Decode(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<Root>":  base + 100,
		"</Root>": base + 101,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// Mocking token sequence: <Root> + "hello" + </Root>
	// "hello" in cl100k_base is [15339]
	tokens := []int{base + 100, 15339, base + 101}

	// Decode adds spaces between tokens
	expected := "<Root> hello </Root>"
	result := tokenizer.Decode(tokens)

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestTokenizer_Tokenize_MissingEndTagInVocab(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<Root>":  base + 1,
		"</Root>": base + 2,
		"<A>":     base + 3,
		// "</A>" is missing
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<Root><A></A></Root>`

	_, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err == nil {
		t.Error("Expected error for missing end tag in vocab, got nil")
	} else {
		expected := "tag </A> not found in vocab"
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Expected error %q, got %q", expected, err.Error())
		}
	}
}

func TestNewTokenizer_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid-vocab-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString("{ invalid json"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	_, err = NewTokenizer(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestNewTokenizer_IDOverlap(t *testing.T) {
	// 50 is definitely overlapping with cl100k_base
	vocab := map[string]int{
		"<Test>": 50,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	_, err := NewTokenizer(vocabPath)
	if err == nil {
		t.Fatal("Expected error due to ID overlap, got nil")
	}

	expectedErrorPart := "overlaps with existing Tiktoken IDs"
	if !strings.Contains(err.Error(), expectedErrorPart) {
		t.Errorf("Expected error to contain %q, got %q", expectedErrorPart, err.Error())
	}
}
