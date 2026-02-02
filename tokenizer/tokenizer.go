package tokenizer

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

type TokenizationResult struct {
	Tokens []int
	Depths []int
}

type Tokenizer struct {
	vocab            map[string]int
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

	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	return &Tokenizer{
		vocab:            vocab,
		contentTokenizer: tke,
	}, nil
}

func (t *Tokenizer) Tokenize(r io.Reader) (*TokenizationResult, error) {
	var tokens []int
	var depths []int
	depth := 0
	decoder := xml.NewDecoder(r)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			tagName := "<" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				tokens = append(tokens, id)
				depths = append(depths, depth)
				depth++
			} else {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				depth--
				tokens = append(tokens, id)
				depths = append(depths, depth)
			} else {
				return nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.CharData:
			content := string(se)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				contentTokens := t.contentTokenizer.Encode(trimmed, nil, nil)
				tokens = append(tokens, contentTokens...)
				for range contentTokens {
					depths = append(depths, depth)
				}
			}
		}
	}
	return &TokenizationResult{
		Tokens: tokens,
		Depths: depths,
	}, nil
}
