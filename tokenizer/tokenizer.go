package tokenizer

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

type Tokenizer struct {
	vocab map[string]int
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

	return &Tokenizer{
		vocab: vocab,
	}, nil
}

func (t *Tokenizer) Tokenize(r io.Reader) ([]int, []int, error) {
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
			return nil, nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			tagName := "<" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				tokens = append(tokens, id)
				depths = append(depths, depth)
				depth++
			} else {
				return nil, nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.EndElement:
			tagName := "</" + se.Name.Local + ">"
			if id, ok := t.vocab[tagName]; ok {
				depth--
				tokens = append(tokens, id)
				depths = append(depths, depth)
			} else {
				return nil, nil, fmt.Errorf("tag %s not found in vocab", tagName)
			}
		case xml.CharData:
			content := string(se)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				// Placeholder for content tokenization
				fmt.Printf("Content: %s\n", trimmed)
			}
		}
	}
	return tokens, depths, nil
}
