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

func TestTransformer_Transform_Basic(t *testing.T) {
	xmlStr := `<div>content</div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
	}

	tr := NewTransformer(vocab)
	root, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div>content</div>`
	if root.String() != expected {
		t.Errorf("expected %s, got %s", expected, root.String())
	}
}

func TestTransformer_Attributes_Registered(t *testing.T) {
	xmlStr := `<div class="foo"></div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		"##class":     3,
		TokenValueEnd: 99,
	}

	tr := NewTransformer(vocab)
	root, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div><__RegisteredAttr><__Key>class</__Key><__Value>foo</__Value></__RegisteredAttr></div>`
	if root.String() != expected {
		t.Errorf("expected %s, got %s", expected, root.String())
	}
}

func TestTransformer_Attributes_Registered_Empty(t *testing.T) {
	xmlStr := `<div class=""></div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		"##class":  3,
		TokenEmpty: 88,
	}

	tr := NewTransformer(vocab)
	root, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div><__RegisteredAttr><__Key>class</__Key><__Value><__Empty></__Empty></__Value></__RegisteredAttr></div>`
	if root.String() != expected {
		t.Errorf("expected %s, got %s", expected, root.String())
	}
}

func TestTransformer_Attributes_Unregistered(t *testing.T) {
	xmlStr := `<div unknown="val"></div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		TokenUnregisteredAttr:    10,
		TokenUnregisteredAttrEnd: 11,
		TokenKey:                 12,
		TokenKeyEnd:              13,
		TokenValue:               14,
		TokenValueEnd:            15,
	}

	tr := NewTransformer(vocab)
	root, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div><__UnregisteredAttr><__Key>unknown</__Key><__Value>val</__Value></__UnregisteredAttr></div>`
	if root.String() != expected {
		t.Errorf("expected %s, got %s", expected, root.String())
	}
}

func TestTransformer_Ordered(t *testing.T) {
	xmlStr := `<div arbor-ordered="true"></div>`
	vocab := map[string]int{"<div>": 1, "</div>": 2}
	tr := NewTransformer(vocab)
	root, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div arbor-ordered="true"></div>`
	if root.String() != expected {
		t.Errorf("expected %s, got %s", expected, root.String())
	}
}

func scanForVocab(r io.Reader) (map[string]int, error) {
	decoder := xml.NewDecoder(r)
	vocab := make(map[string]int)
	id := Cl100kBaseMaxID + 1

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
				attrName := "##" + attr.Name.Local
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
				vocab, err = scanForVocab(f)
				if err != nil {
					t.Fatal(err)
				}
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

				f.Seek(0, 0)
			} else {
				data, err := os.ReadFile(vocabFile)
				if err != nil {
					t.Fatalf("Vocab file %s missing. Run with -update to generate.", vocabFile)
				}
				if err := json.Unmarshal(data, &vocab); err != nil {
					t.Fatal(err)
				}
			}

			tr := NewTransformer(vocab)
			root, err := tr.Transform(f)
			if err != nil {
				t.Fatalf("Transform failed: %v", err)
			}

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

			if StringsDiff(string(expected), string(outBytes)) {
				t.Errorf("content mismatch for %s. Run with -update to fix.", goldenFile)
			}
		})
	}
}

func StringsDiff(a, b string) bool {
	return a != b
}
