package tokenizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		})
	}
}
