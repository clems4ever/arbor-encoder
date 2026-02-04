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

func TestEncoder_RoundTrip(t *testing.T) {
	matches, err := filepath.Glob("testdata/*.html")
	if err != nil {
		t.Fatalf("failed to glob html files: %v", err)
	}

	for _, htmlFile := range matches {
		t.Run(filepath.Base(htmlFile), func(t *testing.T) {
			f, err := os.Open(htmlFile)
			if err != nil {
				t.Fatalf("failed to open html file: %v", err)
			}
			defer f.Close()

			xmlContent, err := ConvertHTMLToXML(f)
			if err != nil {
				t.Fatalf("failed to convert html to xml: %v", err)
			}

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

			tr := NewTransformer(vocab)
			root, err := tr.Transform(strings.NewReader(xmlContent))
			if err != nil {
				t.Fatalf("transform error: %v", err)
			}

			tke, err := tiktoken.GetEncoding("cl100k_base")
			if err != nil {
				t.Fatalf("failed to get tiktoken: %v", err)
			}
			enc := NewEncoder(vocab, tke)

			res, err := enc.Encode(strings.NewReader(root.String()))
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

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

			var buf bytes.Buffer
			decodedRoot.PrettyPrint(&buf, 0)
			actualContent := buf.String()

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
