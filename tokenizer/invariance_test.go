package tokenizer

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// Helper to create a vocab file
func createInvarianceTestVocab(t *testing.T) string {
	vocab := map[string]int{
		"<Root>":       1,
		"</Root>":      2,
		"<List>":       3,
		"</List>":      4,
		"<Item>":       5,
		"</Item>":      6,
		"<Container>":  7,
		"</Container>": 8,
		"<Deep>":       9,
		"</Deep>":      10,
	}
	f, err := os.CreateTemp("", "vocab-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(vocab); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

// TokenPathPair represents a token and its structural path
type TokenPathPair struct {
	Token int
	Path  string // Use string representation for easy comparison/map keys
}

func getPairs(t *testing.T, tokenizer *Tokenizer, xmlStr string) []TokenPathPair {
	res, err := tokenizer.Tokenize(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	var pairs []TokenPathPair
	for i, token := range res.Tokens {
		// We use Sprint for path to make it comparable
		pathStr := fmt.Sprint(res.Paths[i])
		pairs = append(pairs, TokenPathPair{Token: token, Path: pathStr})
	}
	return pairs
}

// signature returns a canonical string representation of the SET of pairs.
// Sorted list of "TokenID:PathString".
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

	// Case 1: Unordered list (ordered="false")
	// Should produce SAME set of (Token, Path) pairs regardless of order.
	xml1 := `<Root><List ordered="false"><Item>A</Item><Item>B</Item></List></Root>`
	xml2 := `<Root><List ordered="false"><Item>B</Item><Item>A</Item></List></Root>`

	pairs1 := getPairs(t, tokenizer, xml1)
	pairs2 := getPairs(t, tokenizer, xml2)

	sig1 := getSetSignature(pairs1)
	sig2 := getSetSignature(pairs2)

	if sig1 != sig2 {
		t.Errorf("Unordered list should have invariant path set.\nSig1: %s\nSig2: %s", sig1, sig2)
	}

	// Case 2: Ordered list (default)
	// Should produce DIFFERENT set of (Token, Path) pairs when swapped.
	xmlOrdered1 := `<Root><List><Item>A</Item><Item>B</Item></List></Root>`
	xmlOrdered2 := `<Root><List><Item>B</Item><Item>A</Item></List></Root>`

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

	// Deep nesting with unordered
	xml1 := `
<Root>
    <Container>
        <Deep ordered="false">
             <Item>A</Item>
             <Item>B</Item>
        </Deep>
    </Container>
</Root>`

	xml2 := `
<Root>
    <Container>
        <Deep ordered="false">
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

	// Scenario 1: Two Levels Unordered
	// Outer List Unordered, Inner Container Unordered.
	// Swapping outer containers -> Same
	// Swapping inner items -> Same
	t.Run("TwoLevelsUnordered", func(t *testing.T) {
		base := `<Root><List ordered="false"><Container ordered="false"><Item>A</Item><Item>B</Item></Container><Container ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List ordered="false"><Container ordered="false"><Item>C</Item><Item>D</Item></Container><Container ordered="false"><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List ordered="false"><Container ordered="false"><Item>B</Item><Item>A</Item></Container><Container ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer unordered elements changed signature")
		}
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner unordered elements changed signature")
		}
	})

	// Scenario 2: Two Levels Ordered
	// Outer List Ordered, Inner Container Ordered.
	// Swapping outer -> Diff
	// Swapping inner -> Diff
	t.Run("TwoLevelsOrdered", func(t *testing.T) {
		base := `<Root><List><Container><Item>A</Item><Item>B</Item></Container><Container><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List><Container><Item>C</Item><Item>D</Item></Container><Container><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List><Container><Item>B</Item><Item>A</Item></Container><Container><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer ordered elements shoud have changed signature")
		}
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner ordered elements should have changed signature")
		}
	})

	// Scenario 3: Ordered of Unordered
	// Outer List Ordered, Inner Container Unordered.
	// Swapping outer -> Diff
	// Swapping inner -> Same
	t.Run("OrderedOfUnordered", func(t *testing.T) {
		base := `<Root><List><Container ordered="false"><Item>A</Item><Item>B</Item></Container><Container ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List><Container ordered="false"><Item>C</Item><Item>D</Item></Container><Container ordered="false"><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List><Container ordered="false"><Item>B</Item><Item>A</Item></Container><Container ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer ordered elements should have changed signature")
		}
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner unordered elements should NOT have changed signature")
		}
	})

	// Scenario 4: Unordered of Ordered
	// Outer List Unordered, Inner Container Ordered.
	// Swapping outer -> Same
	// Swapping inner -> Diff
	t.Run("UnorderedOfOrdered", func(t *testing.T) {
		base := `<Root><List ordered="false"><Container><Item>A</Item><Item>B</Item></Container><Container><Item>C</Item><Item>D</Item></Container></List></Root>`
		swapOuter := `<Root><List ordered="false"><Container><Item>C</Item><Item>D</Item></Container><Container><Item>A</Item><Item>B</Item></Container></List></Root>`
		swapInner := `<Root><List ordered="false"><Container><Item>B</Item><Item>A</Item></Container><Container><Item>C</Item><Item>D</Item></Container></List></Root>`

		sigBase := getSetSignature(getPairs(t, tokenizer, base))
		if sigBase != getSetSignature(getPairs(t, tokenizer, swapOuter)) {
			t.Error("Swapping outer unordered elements should NOT have changed signature")
		}
		if sigBase == getSetSignature(getPairs(t, tokenizer, swapInner)) {
			t.Error("Swapping inner ordered elements SHOULD have changed signature")
		}
	})
}

// -------------------------------------------------------------------------
// Helpers for QKt computation
// -------------------------------------------------------------------------

func pseudoRandomEmbedding(seed int, dim int) []float64 {
	vec := make([]float64, dim)
	for i := 0; i < dim; i++ {
		// Simple determinstic mixing
		val := float64(((seed*997)+(i*727))%10000) / 10000.0
		vec[i] = val
	}
	return vec
}

func computeFinalEmbeddings(tokens []int, paths [][]int) [][]float64 {
	dim := 8 // Small dimension for test
	embeddings := make([][]float64, len(tokens))

	for i, token := range tokens {
		// 1. Token Embedding
		// Use a pseudo-random generator to simulate a learned embedding matrix
		// Large offset ensures separation from path seeds
		tokenVec := pseudoRandomEmbedding(token+1000000, dim)

		finalVec := make([]float64, dim)
		copy(finalVec, tokenVec)

		// 2. Add Structural Path Embeddings
		// This directly mimics the model logic:
		// Emb = TokenEmb(t) + Sum_level( LevelEmb[level](path[level]) )
		path := paths[i]
		for depth, idx := range path {
			// Simulate learning a distinct embedding vector for each (depth, index)
			// Seed = hash(depth, index)
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

// embeddingsToCanonicalString converts the batch of embeddings into a string signature.
// If orderInvariant is true, it sorts the rows (treating the batch as a set),
// which allows verifying that unordered inputs produce identical sets of embeddings.
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

			emb1 := computeFinalEmbeddings(res1.Tokens, res1.Paths)
			emb2 := computeFinalEmbeddings(res2.Tokens, res2.Paths)

			// We compare the *sets* of embedding vectors produced.
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

	// 1. Unordered List - Swapping items should yield SAME set of embeddings
	runComparison(t, "Unordered",
		`<Root><List ordered="false"><Item>A</Item><Item>B</Item></List></Root>`,
		`<Root><List ordered="false"><Item>B</Item><Item>A</Item></List></Root>`,
		true,
	)

	// 2. Ordered List - Swapping items should yield DIFFERENT set of embeddings
	runComparison(t, "Ordered",
		`<Root><List><Item>A</Item><Item>B</Item></List></Root>`,
		`<Root><List><Item>B</Item><Item>A</Item></List></Root>`,
		false,
	)

	// 3. Mixed: Ordered of Unordered - Swapping outer (ordered) changes set
	runComparison(t, "OrderedOfUnordered_SwapOuter",
		`<Root><List><Container ordered="false"><Item>A</Item><Item>B</Item></Container><Container ordered="false"><Item>C</Item><Item>D</Item></Container></List></Root>`,
		`<Root><List><Container ordered="false"><Item>C</Item><Item>D</Item></Container><Container ordered="false"><Item>A</Item><Item>B</Item></Container></List></Root>`,
		false,
	)

	// 4. Mixed: Unordered of Ordered - Swapping outer (unordered) keeps set invariant
	runComparison(t, "UnorderedOfOrdered_SwapOuter",
		`<Root><List ordered="false"><Container><Item>A</Item><Item>B</Item></Container><Container><Item>C</Item><Item>D</Item></Container></List></Root>`,
		`<Root><List ordered="false"><Container><Item>C</Item><Item>D</Item></Container><Container><Item>A</Item><Item>B</Item></Container></List></Root>`,
		true,
	)
}
