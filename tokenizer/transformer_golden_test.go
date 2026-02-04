package tokenizer

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func scanForVocab(r io.Reader) (map[string]int, error) {
	decoder := xml.NewDecoder(r)
	vocab := make(map[string]int)
	id := 1

	// Add special tags required by Transformer for fallback or delimiters
	special := []string{
		TokenRegisteredAttr,
		TokenUnregisteredAttr, TokenUnregisteredAttrEnd,
		TokenKey, TokenKeyEnd,
		TokenValue, TokenValueEnd,
		TokenEmpty,
	}
	for _, s := range special {
		vocab[s] = id
		id++
	}

	for {
		t, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch se := t.(type) {
		case xml.StartElement:
			tag := "<" + se.Name.Local + ">"
			if _, ok := vocab[tag]; !ok {
				vocab[tag] = id
				id++
			}
			for _, attr := range se.Attr {
				if attr.Name.Local == ArborOrderedAttribute {
					continue
				}
				// Do not register "data-*" attributes to allow testing unregistered attribute path
				if strings.HasPrefix(attr.Name.Local, "data-") {
					continue
				}
				// Do not register "lang" to test unregistered path for standard attributes
				if attr.Name.Local == "lang" {
					continue
				}
				attrName := "@" + attr.Name.Local
				if _, ok := vocab[attrName]; !ok {
					vocab[attrName] = id
					id++
				}
			}
		case xml.EndElement:
			tag := "</" + se.Name.Local + ">"
			if _, ok := vocab[tag]; !ok {
				vocab[tag] = id
				id++
			}
		}
	}
	return vocab, nil
}

func TestTransformer_Golden(t *testing.T) {
	matches, err := filepath.Glob("testdata/*_golden.xml")
	if err != nil {
		t.Fatal(err)
	}

	for _, inFile := range matches {
		t.Run(filepath.Base(inFile), func(t *testing.T) {
			f, err := os.Open(inFile)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			vocabFile := strings.TrimSuffix(inFile, ".xml") + "_vocab.json"
			var vocab map[string]int

			if *update {
				// 1. Build Vocab from file to ensure we cover all tags/attributes
				// This simulates a "complete" vocab scenario.
				vocab, err = scanForVocab(f)
				if err != nil {
					t.Fatal(err)
				}
				// Save vocab
				var buf bytes.Buffer
				enc := json.NewEncoder(&buf)
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				if err := enc.Encode(vocab); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(vocabFile, buf.Bytes(), 0644); err != nil {
					t.Fatal(err)
				}
				
				f.Seek(0, 0) // Rewind
			} else {
				// Read vocab
				data, err := os.ReadFile(vocabFile)
				if err != nil {
					t.Fatalf("Vocab file %s missing. Run with -update to generate.", vocabFile)
				}
				if err := json.Unmarshal(data, &vocab); err != nil {
					t.Fatal(err)
				}
			}

			// 2. Transform
			tr := NewTransformer(vocab)
			root, err := tr.Transform(f)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

			// 3. Serialize and Indent
			var buf bytes.Buffer
			root.PrettyPrint(&buf, 0)
			outBytes := buf.Bytes()

			goldenFile := strings.TrimSuffix(inFile, ".xml") + "_virtual.xml"

			if *update {
				if err := os.WriteFile(goldenFile, outBytes, 0644); err != nil {
					t.Fatal(err)
				}
			}

			expected, err := os.ReadFile(goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file %s missing, run with -update to generate", goldenFile)
				}
				t.Fatal(err)
			}

			// Normalize line endings for comparison just in case
			if StringsDiff(string(expected), string(outBytes)) {
				t.Errorf("content mismatch for %s. Run with -update to fix.", goldenFile)
			}
		})
	}
}

func StringsDiff(a, b string) bool {
	// Simple equality for now
	return a != b
}
