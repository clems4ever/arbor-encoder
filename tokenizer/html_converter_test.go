package tokenizer

import (
	"os"
	"strings"
	"testing"
)

func TestConvertHTMLToXML(t *testing.T) {
	// Read golden files from testdata
	inputFile := "testdata/example.html"
	goldenFile := "testdata/example_golden.xml"

	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read input file %s: %v", inputFile, err)
	}

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
		t.Errorf("Result does not match golden file.\nExpected len: %d\nActual len: %d\nSnippet Expected: %s\nSnippet Actual: %s", 
			len(expected), len(actual), expected[:100], actual[:100])
	}
}
