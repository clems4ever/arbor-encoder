package tokenizer

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func collectVocabFromXML(t *testing.T, xmlData string) map[string]int {
	decoder := xml.NewDecoder(strings.NewReader(xmlData))
	vocab := map[string]int{
		TokenRegisteredAttr:      Cl100kBaseMaxID + 1,
		TokenUnregisteredAttr:    Cl100kBaseMaxID + 2,
		TokenUnregisteredAttrEnd: Cl100kBaseMaxID + 3,
		TokenKey:                 Cl100kBaseMaxID + 4,
		TokenKeyEnd:              Cl100kBaseMaxID + 5,
		TokenValue:               Cl100kBaseMaxID + 6,
		TokenValueEnd:            Cl100kBaseMaxID + 7,
		TokenEmpty:       Cl100kBaseMaxID + 7,
	}

	nextID := Cl100kBaseMaxID + 100

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Vocab collector failed: %v", err)
		}

		switch se := tok.(type) {
		case xml.StartElement:
			startTag := "<" + se.Name.Local + ">"
			if _, ok := vocab[startTag]; !ok {
				vocab[startTag] = nextID
				nextID++
			}
			endTag := "</" + se.Name.Local + ">"
			if _, ok := vocab[endTag]; !ok {
				vocab[endTag] = nextID
				nextID++
			}
			for _, attr := range se.Attr {
				attrName := "@" + attr.Name.Local
				if _, ok := vocab[attrName]; !ok {
					vocab[attrName] = nextID
					nextID++
				}
			}
		}
	}
	return vocab
}

func TestConvertHTMLToXML(t *testing.T) {
	files, err := filepath.Glob("testdata/*.html")
	if err != nil {
		t.Fatalf("Failed to glob html files: %v", err)
	}

	for _, inputFile := range files {
		t.Run(inputFile, func(t *testing.T) {
			goldenFile := strings.TrimSuffix(inputFile, ".html") + "_golden.xml"

			inputBytes, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("Failed to read input file %s: %v", inputFile, err)
			}

			// If golden file doesn't exist, we might skip or fail.
			// For now let's read it.
			expectedBytes, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v", goldenFile, err)
			}
			expected := string(expectedBytes)

			// Run conversion
			actual, err := ConvertHTMLToXML(strings.NewReader(string(inputBytes)))
			if err != nil {
				t.Fatalf("ConvertHTMLToXML failed: %v", err)
			}

			if actual != expected {
				t.Errorf("Result does not match golden file for %s.\nExpected len: %d\nActual len: %d\n",
					inputFile, len(expected), len(actual))
			}

			// --- Round-trip Tokenization Check ---

			// 1. Build a dynamic vocab from the XML content
			vocab := collectVocabFromXML(t, actual)
			vocabPath := createTempVocab(t, vocab)
			defer os.Remove(vocabPath)

			// 2. Initialize Tokenizer
			tok, err := NewTokenizer(vocabPath)
			if err != nil {
				t.Fatalf("Failed to create tokenizer: %v", err)
			}

			// 3. Tokenize
			res, err := tok.Tokenize(strings.NewReader(actual))
			if err != nil {
				t.Fatalf("Tokenization failed: %v", err)
			}

			// 4. Decode structure
			decodedStruct, err := tok.DecodeXML(res.Tokens)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			// 5. Parse original XML to structure for comparison
			expectedStruct, err := parseXMLToElement(actual)
			if err != nil {
				t.Fatalf("Failed to parse actual XML to Element: %v", err)
			}

			// 6. Compare
			elementsMatch(t, expectedStruct, decodedStruct)
		})
	}
}
