package tokenizer

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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

	paths := res.PaddedPaths
	if countValid(paths[0]) != 1 {
		t.Errorf("Expected path length 1 for root, got %d", countValid(paths[0]))
	}
	if countValid(paths[1]) != 2 {
		t.Errorf("Expected path length 2 for child, got %d", countValid(paths[1]))
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

	findIndex := func(target int) int {
		for i, tok := range tokens {
			if tok == target {
				return i
			}
		}
		return -1
	}

	checks := []struct {
		tokenID     int
		expectedLen int
		name        string
	}{
		{200010, 1, "<Root>"},
		{200012, 2, "<Level1>"},
		{200014, 3, "<Level2>"},
		{200016, 4, "<Level3>"},
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

func TestTokenizer_Attributes(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":     base + 100,
		"</City>":    base + 101,
		"<School>":   base + 102,
		"</School>":  base + 103,
		"##name":     base + 104,
		"##zip":      base + 105,
		"</__Value>": base + 106,
		"<__Empty/>": base + 107,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<City name="Paris" zip="75000"><School>S1</School></City>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	tokens := res.Tokens
	paths := res.PaddedPaths

	if tokens[0] != base+100 {
		t.Errorf("Expected City token at 0")
	}
	if len(paths[0]) != 3 || paths[0][0] != 0 {
		t.Errorf("City path mismatch: %v", paths[0])
	}

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

	parisIdx := nameIdx + 1
	if len(paths[parisIdx]) != 3 || paths[parisIdx][2] != 0 {
		t.Errorf("Paris value path invalid, expected ending in 0, got %v", paths[parisIdx])
	}

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

	if paths[schoolIdx][1] != 1 {
		t.Errorf("School path invalid, expected [0, 1], got %v", paths[schoolIdx])
	}
}

func TestTokenizer_Attributes_UnorderedParent(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":     base + 100,
		"</City>":    base + 101,
		"<Item>":     base + 102,
		"</Item>":    base + 103,
		"##attr":     base + 104,
		"</__Value>": base + 105,
		"<__Empty/>": base + 106,
	}
	vocabPath := createTempVocab(t, vocab)
	defer os.Remove(vocabPath)

	tokenizer, _ := NewTokenizer(vocabPath)

	xmlContent := `<City arbor-ordered="false" attr="val"><Item>A</Item><Item>B</Item></City>`

	res, err := tokenizer.Tokenize(strings.NewReader(xmlContent))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	paths := res.PaddedPaths
	tokens := res.Tokens

	var itemIndices []int
	for i, tok := range tokens {
		if tok == base+102 {
			itemIndices = append(itemIndices, i)
		}
	}

	if len(itemIndices) != 2 {
		t.Fatal("Expected 2 Item tokens")
	}

	for _, idx := range itemIndices {
		p := paths[idx]
		if p[1] != 1 {
			t.Errorf("Item path invalid for unordered container. Expected index 1, got path %v", p)
		}
	}

	attrIdx := -1
	for i, tok := range tokens {
		if tok == base+104 {
			attrIdx = i
			break
		}
	}
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
	} else if !strings.Contains(err.Error(), "attribute ##unknown not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestTokenizer_UnregisteredAttributes(t *testing.T) {
	base := 200000
	vocab := map[string]int{
		"<City>":                base + 100,
		"</City>":               base + 101,
		"@name":                 base + 102,
		"<__UnregisteredAttr>":  base + 200,
		"</__UnregisteredAttr>": base + 201,
		"<__Key>":               base + 202,
		"</__Key>":              base + 203,
		"<__Value>":             base + 204,
		"</__Value>":            base + 205,
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

	startUnknownIdx := findSequence([]int{base + 200, base + 202})
	if startUnknownIdx == -1 {
		t.Fatalf("Did not find Start of Unknown Attribute structure (tokens %d, %d)", base+200, base+202)
	}

	if len(paths[startUnknownIdx]) < 2 {
		t.Errorf("Path too short for AttrPair")
	}

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

	valStartPath := paths[valStartIdx]
	if len(valStartPath) < 3 || valStartPath[0] != 0 || valStartPath[1] != 0 || valStartPath[2] != 1 {
		t.Errorf("ValueStart path incorrect: %v. Expected prefix [0, 0, 1]", valStartPath)
	}
}

func createInvarianceTestVocab(t *testing.T) string {
	base := 200000
	vocab := map[string]int{
		"<Root>":       base + 1,
		"</Root>":      base + 2,
		"<List>":       base + 3,
		"</List>":      base + 4,
		"<Item>":       base + 5,
		"</Item>":      base + 6,
		"<Container>":  base + 7,
		"</Container>": base + 8,
		"<Deep>":       base + 9,
		"</Deep>":      base + 10,
	}
	return createTempVocab(t, vocab)
}

type TokenPathPair struct {
	Token int
	Path  string
}

func getPairs(t *testing.T, tokenizer *Tokenizer, xmlStr string) []TokenPathPair {
	res, err := tokenizer.Tokenize(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	var pairs []TokenPathPair
	for i, token := range res.Tokens {
		pathStr := fmt.Sprint(res.PaddedPaths[i])
		pairs = append(pairs, TokenPathPair{Token: token, Path: pathStr})
	}
	return pairs
}

func getSetSignature(pairs []TokenPathPair) string {
	var s []string
	for _, p := range pairs {
		s = append(s, fmt.Sprintf("%d:%s", p.Token, p.Path))
	}
	sort.Strings(s)
	return strings.Join(s, "|")
}

func TestOrderInvariance(t *testing.T) {
	vocabPath := createInvarianceTestVocab(t)
	defer os.Remove(vocabPath)

	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatal(err)
	}

	xml1 := `<Root><List arbor-ordered="false"><Item>A</Item><Item>B</Item></List></Root>`
	xml2 := `<Root><List arbor-ordered="false"><Item>B</Item><Item>A</Item></List></Root>`

	pairs1 := getPairs(t, tokenizer, xml1)
	pairs2 := getPairs(t, tokenizer, xml2)

	sig1 := getSetSignature(pairs1)
	sig2 := getSetSignature(pairs2)

	if sig1 != sig2 {
		t.Errorf("Unordered list should have invariant path set.\nSig1: %s\nSig2: %s", sig1, sig2)
	}

	xmlOrdered1 := `<Root><List arbor-ordered="true"><Item>A</Item><Item>B</Item></List></Root>`
	xmlOrdered2 := `<Root><List arbor-ordered="true"><Item>B</Item><Item>A</Item></List></Root>`

	pairsO1 := getPairs(t, tokenizer, xmlOrdered1)
	pairsO2 := getPairs(t, tokenizer, xmlOrdered2)

	sigO1 := getSetSignature(pairsO1)
	sigO2 := getSetSignature(pairsO2)

	if sigO1 == sigO2 {
		t.Errorf("Ordered list should have VARIANT path set.\nSig1: %s\nSig2: %s", sigO1, sigO2)
	}
}

func TestDeepOrderInvariance(t *testing.T) {
	vocabPath := createInvarianceTestVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, _ := NewTokenizer(vocabPath)

	xml1 := `
<Root>
    <Container>
        <Deep arbor-ordered="false">
             <Item>A</Item>
             <Item>B</Item>
        </Deep>
    </Container>
</Root>`

	xml2 := `
<Root>
    <Container>
        <Deep arbor-ordered="false">
             <Item>B</Item>
             <Item>A</Item>
        </Deep>
    </Container>
</Root>`

	if getSetSignature(getPairs(t, tokenizer, xml1)) != getSetSignature(getPairs(t, tokenizer, xml2)) {
		t.Error("Deep unordered list failed invariance check")
	}
}

func TestNestedInvarianceLevels(t *testing.T) {
	vocabPath := createInvarianceTestVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, _ := NewTokenizer(vocabPath)

	t.Run("TwoLevelsUnordered", func(t *testing.T) {
		base := `<Root><List arbor-ordered="false"><Container arbor-ordered="false"><Item>A</Item><Item>B</Item></Container><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List arbor-ordered="false"><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container><Container arbor-ordered="false"><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List arbor-ordered="false"><Container arbor-ordered="false"><Item>B</Item><Item>A</Item></Container><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer unordered elements changed signature")
		}
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner unordered elements changed signature")
		}
	})

	t.Run("TwoLevelsOrdered", func(t *testing.T) {
		base := `<Root><List arbor-ordered="true"><Container arbor-ordered="true"><Item>A</Item><Item>B</Item></Container><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List arbor-ordered="true"><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container><Container arbor-ordered="true"><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List arbor-ordered="true"><Container arbor-ordered="true"><Item>B</Item><Item>A</Item></Container><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer ordered elements shoud have changed signature")
		}
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner ordered elements should have changed signature")
		}
	})

	t.Run("OrderedOfUnordered", func(t *testing.T) {
		base := `<Root><List arbor-ordered="true"><Container arbor-ordered="false"><Item>A</Item><Item>B</Item></Container><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List arbor-ordered="true"><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container><Container arbor-ordered="false"><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List arbor-ordered="true"><Container arbor-ordered="false"><Item>B</Item><Item>A</Item></Container><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer ordered elements should have changed signature")
		}
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner unordered elements should NOT have changed signature")
		}
	})

	t.Run("UnorderedOfOrdered", func(t *testing.T) {
		base := `<Root><List arbor-ordered="false"><Container arbor-ordered="true"><Item>A</Item><Item>B</Item></Container><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List arbor-ordered="false"><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container><Container arbor-ordered="true"><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List arbor-ordered="false"><Container arbor-ordered="true"><Item>B</Item><Item>A</Item></Container><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer unordered elements should NOT have changed signature")
		}
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner ordered elements SHOULD have changed signature")
		}
	})
}

func pseudoRandomEmbedding(seed int, dim int) []float64 {
	vec := make([]float64, dim)
	for i := 0; i < dim; i++ {
		val := float64(((seed*997)+(i*727))%10000) / 10000.0
		vec[i] = val
	}
	return vec
}

func computeFinalEmbeddings(tokens []int, paths [][]int) [][]float64 {
	dim := 8
	embeddings := make([][]float64, len(tokens))

	for i, token := range tokens {
		tokenVec := pseudoRandomEmbedding(token+1000000, dim)

		finalVec := make([]float64, dim)
		copy(finalVec, tokenVec)

		path := paths[i]
		for depth, idx := range path {
			coordSeed := (depth+1)*1000 + idx
			levelVec := pseudoRandomEmbedding(coordSeed, dim)

			for k := 0; k < dim; k++ {
				finalVec[k] += levelVec[k]
			}
		}
		embeddings[i] = finalVec
	}
	return embeddings
}

func embeddingsToCanonicalString(embs [][]float64, orderInvariant bool) string {
	rowStrings := make([]string, len(embs))
	for i, row := range embs {
		vals := make([]string, len(row))
		for j, v := range row {
			vals[j] = fmt.Sprintf("%.5f", v)
		}
		rowStrings[i] = "[" + strings.Join(vals, ",") + "]"
	}

	if orderInvariant {
		sort.Strings(rowStrings)
	}
	return strings.Join(rowStrings, "\n")
}

func TestEmbeddingComputationInvariance(t *testing.T) {
	vocabPath := createInvarianceTestVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, _ := NewTokenizer(vocabPath)

	runComparison := func(t *testing.T, label string, xml1, xml2 string, expectInvariantSet bool) {
		t.Helper()
		t.Run(label, func(t *testing.T) {
			res1, err := tokenizer.Tokenize(strings.NewReader(xml1))
			if err != nil {
				t.Fatal(err)
			}
			res2, err := tokenizer.Tokenize(strings.NewReader(xml2))
			if err != nil {
				t.Fatal(err)
			}

			emb1 := computeFinalEmbeddings(res1.Tokens, res1.PaddedPaths)
			emb2 := computeFinalEmbeddings(res2.Tokens, res2.PaddedPaths)

			sig1 := embeddingsToCanonicalString(emb1, true)
			sig2 := embeddingsToCanonicalString(emb2, true)

			areEqual := (sig1 == sig2)

			if expectInvariantSet && !areEqual {
				t.Errorf("Expected embeddings to be identical (as a set), but they differed.\nSet1:\n%s\nSet2:\n%s", sig1, sig2)
			}
			if !expectInvariantSet && areEqual {
				t.Errorf("Expected embeddings to be DIFFERENT (as a set), but they were identical.\nSet:\n%s", sig1)
			}
		})
	}

	runComparison(t, "Unordered",
		`<Root><List arbor-ordered="false"><Item>A</Item><Item>B</Item></List></Root>`,
		`<Root><List arbor-ordered="false"><Item>B</Item><Item>A</Item></List></Root>`,
		true,
	)

	runComparison(t, "Ordered",
		`<Root><List arbor-ordered="true"><Item>A</Item><Item>B</Item></List></Root>`,
		`<Root><List arbor-ordered="true"><Item>B</Item><Item>A</Item></List></Root>`,
		false,
	)

	runComparison(t, "OrderedOfUnordered_SwapOuter",
		`<Root><List arbor-ordered="true"><Container arbor-ordered="false"><Item>A</Item><Item>B</Item></Container><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`,
		`<Root><List arbor-ordered="true"><Container arbor-ordered="false"><Item>C</Item><Item>D</Item></Container><Container arbor-ordered="false"><Item>A</Item><Item>B</Item></Container></List></Root>`,
		false,
	)

	runComparison(t, "UnorderedOfOrdered_SwapOuter",
		`<Root><List arbor-ordered="false"><Container arbor-ordered="true"><Item>A</Item><Item>B</Item></Container><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container></List></Root>`,
		`<Root><List arbor-ordered="false"><Container arbor-ordered="true"><Item>C</Item><Item>D</Item></Container><Container arbor-ordered="true"><Item>A</Item><Item>B</Item></Container></List></Root>`,
		true,
	)
}

func createComprehensiveVocab(t *testing.T) string {
	base := 200000
	vocab := map[string]int{
		"<Root>":                base + 1,
		"</Root>":               base + 2,
		"<Child>":               base + 3,
		"</Child>":              base + 4,
		"<SubChild>":            base + 5,
		"</SubChild>":           base + 6,
		"<Leaf>":                base + 7,
		"</Leaf>":               base + 8,
		"@id":                   base + 100,
		"@type":                 base + 101,
		"@extra":                base + 102,
		"<__UnregisteredAttr>":  base + 200,
		"</__UnregisteredAttr>": base + 201,
		"<__Key>":               base + 202,
		"</__Key>":              base + 203,
		"<__Value>":             base + 204,
		"</__Value>":            base + 205,
		"<__Empty/>":            base + 206,
		"<__RegisteredAttr>":    base + 207,
		"</__RegisteredAttr>":   base + 208,
	}
	return createTempVocab(t, vocab)
}

func TestStructureLogic(t *testing.T) {
	vocabPath := createComprehensiveVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, _ := NewTokenizer(vocabPath)

	inputUnordered := `<Root><Child>A</Child><Child>B</Child></Root>`

	resU, _ := tokenizer.Tokenize(strings.NewReader(inputUnordered))

	var childIndices []int
	for i, tok := range resU.Tokens {
		if tok == 200003 {
			p := resU.PaddedPaths[i]
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

	inputOrdered := `<Root arbor-ordered="true"><Child>A</Child><Child>B</Child></Root>`
	resO, _ := tokenizer.Tokenize(strings.NewReader(inputOrdered))

	childIndices = []int{}
	for i, tok := range resO.Tokens {
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
