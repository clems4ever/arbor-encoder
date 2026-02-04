package tokenizer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

const (
	ArborOrderedAttribute    = "arbor-ordered"
	TokenRegisteredAttr      = "<__RegisteredAttr>"
	TokenUnregisteredAttr    = "<__UnregisteredAttr>"
	TokenUnregisteredAttrEnd = "</__UnregisteredAttr>"
	TokenKey                 = "<__Key>"
	TokenKeyEnd              = "</__Key>"
	TokenValue               = "<__Value>"
	TokenValueEnd            = "</__Value>"
	TokenEmpty               = "<__Empty/>"
	// Cl100kBaseMaxID is the rough upper bound of cl100k_base vocab.
	// The exact size is around 100277. We use 100500 to be safe.
	Cl100kBaseMaxID = 100500
)

type TokenizationResult struct {
	Tokens      []int
	PaddedPaths [][]int
}

type Tokenizer struct {
	vocab            map[string]int
	vocabInv         map[int]string
	contentTokenizer *tiktoken.Tiktoken
}

func NewTokenizer(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open vocab file: %w", err)
	}
	defer f.Close()

	var vocab map[string]int
	if err := json.NewDecoder(f).Decode(&vocab); err != nil {
		return nil, fmt.Errorf("failed to decode vocab file: %w", err)
	}

	vocabInv := make(map[int]string)
	for k, v := range vocab {
		if v < Cl100kBaseMaxID {
			return nil, fmt.Errorf("token ID %d for tag %s overlaps with existing Tiktoken IDs (max %d). Please use IDs greater than %d to avoid conflicts", v, k, Cl100kBaseMaxID, Cl100kBaseMaxID)
		}
		vocabInv[v] = k
	}

	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	return &Tokenizer{
		vocab:            vocab,
		vocabInv:         vocabInv,
		contentTokenizer: tke,
	}, nil
}

func (t *Tokenizer) Tokenize(r io.Reader) (*TokenizationResult, error) {
	transformer := NewTransformer(t.vocab)
	rootElement, err := transformer.Transform(r)
	if err != nil {
		return nil, err
	}

	encoder := NewEncoder(t.vocab, t.contentTokenizer)
	// Serialize Element to string and pass to Encoder.
	// Ideally Encoder could traverse Element directly, but sticking to XML stream interface for now.
	// Element.String() produces valid XML.
	return encoder.Encode(strings.NewReader(rootElement.String()))
}

// getPaddedPaths returns the paths as a 2D matrix.
// It pads shorter paths with padValue (usually -1).
func getPaddedPaths(paths [][]int, maxDepth int, padValue int) [][]int {
	// If maxDepth is 0, find the actual max depth in the data
	if maxDepth == 0 {
		for _, p := range paths {
			if len(p) > maxDepth {
				maxDepth = len(p)
			}
		}
	}

	tensor := make([][]int, len(paths))

	for i, p := range paths {
		row := make([]int, maxDepth)
		for j := 0; j < maxDepth; j++ {
			if j < len(p) {
				row[j] = p[j]
			} else {
				row[j] = padValue
			}
		}
		tensor[i] = row
	}

	return tensor
}

func (t *Tokenizer) Decode(tokens []int) string {
	var parts []string
	for _, token := range tokens {
		if tag, ok := t.vocabInv[token]; ok {
			parts = append(parts, tag)
		} else {
			val := t.contentTokenizer.Decode([]int{token})
			parts = append(parts, val)
		}
	}
	return strings.Join(parts, " ")
}
