package tokenizer

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkoukk/tiktoken-go"
)

func TestWebsiteRoundTrip(t *testing.T) {
	// Find all HTML files in testdata
	matches, err := filepath.Glob("testdata/*.html")
	if err != nil {
		t.Fatalf("failed to glob html files: %v", err)
	}

	for _, htmlFile := range matches {
		t.Run(filepath.Base(htmlFile), func(t *testing.T) {
			// 1. Read the HTML file
			f, err := os.Open(htmlFile)
			if err != nil {
				t.Fatalf("failed to open html file: %v", err)
			}
			defer f.Close()

			// 2. Convert HTML to XML (standardization)
			xmlContent, err := ConvertHTMLToXML(f)
			if err != nil {
				t.Fatalf("failed to convert html to xml: %v", err)
			}

			// 3. Build Vocab dynamically
			vocab := make(map[string]int)
			id := Cl100kBaseMaxID + 1

			// Add special tokens mandatory for the system
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

			// Scan the XML content to populate vocab with tags
			decoder := xml.NewDecoder(strings.NewReader(xmlContent))
			for {
				tok, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("xml decode error: %v", err)
				}
				switch se := tok.(type) {
				case xml.StartElement:
					tagName := "<" + se.Name.Local + ">"
					if _, ok := vocab[tagName]; !ok {
						vocab[tagName] = id
						id++
					}
					endTagName := "</" + se.Name.Local + ">"
					if _, ok := vocab[endTagName]; !ok {
						vocab[endTagName] = id
						id++
					}
					// Register some attributes to test "Registered Attribute" path
					// Let's register "class", "href", "id" to ensure they are compressed
					for _, attr := range se.Attr {
						attrName := "@" + attr.Name.Local
						if attr.Name.Local == "class" || attr.Name.Local == "href" || attr.Name.Local == "id" {
							if _, ok := vocab[attrName]; !ok {
								vocab[attrName] = id
								id++
							}
						}
					}
				}
			}

			// 4. Create Transformer and Transform
			tr := NewTransformer(vocab)
			root, err := tr.Transform(strings.NewReader(xmlContent))
			if err != nil {
				t.Fatalf("transform error: %v", err)
			}

			// 5. Create Tokenizer/Encoder components
			tke, err := tiktoken.GetEncoding("cl100k_base")
			if err != nil {
				t.Fatalf("failed to get tiktoken: %v", err)
			}
			enc := NewEncoder(vocab, tke)

			// 6. Encode
			res, err := enc.Encode(strings.NewReader(root.String()))
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			// 7. Decode
			vocabInv := make(map[int]string)
			for k, v := range vocab {
				vocabInv[v] = k
			}

			tok := &Tokenizer{
				vocab:            vocab,
				vocabInv:         vocabInv,
				contentTokenizer: tke,
			}

			decodedRoot, err := tok.DecodeXML(res.Tokens)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}

			// 8. Pretty Print Result
			var buf bytes.Buffer
			decodedRoot.PrettyPrint(&buf, 0)
			actualContent := buf.String()

			// 9. Compare with Golden File
			goldenFuncName := strings.TrimSuffix(filepath.Base(htmlFile), ".html")
			goldenFile := filepath.Join("testdata", goldenFuncName+"_decoded.xml")

			if *update {
				if err := os.WriteFile(goldenFile, []byte(actualContent), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			expectedContent, err := os.ReadFile(goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("golden file %s missing, run with -update to generate", goldenFile)
				}
				t.Fatalf("failed to read golden file: %v", err)
			}

			if string(expectedContent) != actualContent {
				t.Errorf("Round trip mismatch against golden file %s", goldenFile)
			}
		})
	}
}
