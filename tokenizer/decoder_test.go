package tokenizer

import (
	"encoding/xml"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Helper to check if two Element structures are effectively identical
func elementsMatch(t *testing.T, expected, actual *Element) {
	if expected.Name != actual.Name {
		t.Errorf("Element name mismatch: expected %s, got %s", expected.Name, actual.Name)
	}

	// Attribute count might differ if we have default attributes or something,
	// but generally should be same for roundtrip of explicit XML.
	if len(expected.Attributes) != len(actual.Attributes) {
		t.Errorf("Attribute count mismatch for %s: expected %d, got %d", expected.Name, len(expected.Attributes), len(actual.Attributes))
	}

	// Compare attributes (order independent)
	expAttrs := make(map[string]string)
	for _, a := range expected.Attributes {
		expAttrs[a.Name.Local] = a.Value
	}
	actAttrs := make(map[string]string)
	for _, a := range actual.Attributes {
		actAttrs[a.Name.Local] = a.Value
	}

	if !reflect.DeepEqual(expAttrs, actAttrs) {
		t.Errorf("Attributes mismatch for %s: expected %v, got %v", expected.Name, expAttrs, actAttrs)
	}

	if len(expected.Children) != len(actual.Children) {
		t.Errorf("Children count mismatch for %s: expected %d, got %d", expected.Name, len(expected.Children), len(actual.Children))
		return
	}

	for i, c := range expected.Children {
		switch expChild := c.(type) {
		case *Element:
			actChild, ok := actual.Children[i].(*Element)
			if !ok {
				t.Errorf("Child type mismatch at index %d: expected *Element, got %T", i, actual.Children[i])
				continue
			}
			elementsMatch(t, expChild, actChild)
		case string:
			actChild, ok := actual.Children[i].(string)
			if !ok {
				t.Errorf("Child type mismatch at index %d: expected string, got %T", i, actual.Children[i])
				continue
			}
			// Trim spaces for comparison stability
			if strings.TrimSpace(expChild) != strings.TrimSpace(actChild) {
				t.Errorf("Content mismatch at index %d for %s: expected '%s', got '%s'", i, expected.Name, expChild, actChild)
			}
		}
	}
}

// Convert input generic XML string to *Element structure using encoding/xml
// This establishes the "ground truth" expectation.
func parseXMLToElement(data string) (*Element, error) {
	decoder := xml.NewDecoder(strings.NewReader(data))
	var root *Element
	var stack []*Element

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			el := &Element{Name: t.Name.Local}
			for _, a := range t.Attr {
				// Ignore custom tokenizer control attributes if they appear in input
				if a.Name.Local == "arbor-ordered" {
					continue
				}
				el.Attributes = append(el.Attributes, a)
			}

			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, el)
			} else {
				root = el
			}
			stack = append(stack, el)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			content := string(t)
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				if len(stack) > 0 {
					// Merge consecutive text nodes if needed (simplified here)
					current := stack[len(stack)-1]
					current.Children = append(current.Children, trimmed)
				}
			}
		}
	}
	return root, nil
}

func TestDecoder_RoundTrip_Extensive(t *testing.T) {
	// Uses createComprehensiveVocab from comprehensive_test.go which MUST be in the same package
	vocabPath := createComprehensiveVocab(t)
	defer os.Remove(vocabPath)
	tokenizer, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name  string
		input string
	}{
		// 1. Basic Structure
		{
			name:  "Simple Root",
			input: `<Root></Root>`,
		},
		{
			name:  "Simple Content",
			input: `<Root>Hello World</Root>`,
		},

		// 2. Attributes (Registered vs Unregistered)
		{
			name:  "Single Registered Attribute",
			input: `<Root id="123"></Root>`,
		},
		{
			name:  "Single Unregistered Attribute",
			input: `<Root unknown="value"></Root>`,
		},
		{
			name:  "Mixed Attributes",
			input: `<Root id="123" unknown="value" type="test"></Root>`,
		},
		{
			name:  "Attributes with Special Chars",
			input: `<Root unknown="a&amp;b"></Root>`,
		},

		// 3. Nesting
		{
			name:  "Simple Nesting",
			input: `<Root><Child>Content</Child></Root>`,
		},
		{
			name:  "Multiple Children",
			input: `<Root><Child>A</Child><Child>B</Child></Root>`,
		},
		{
			name:  "Double Nesting",
			input: `<Root><Child><SubChild>Deep</SubChild></Child></Root>`,
		},

		// 4. Mixed Content (Text and Elements)
		{
			name:  "Mixed Content 1",
			input: `<Root>Start<Child>Middle</Child>End</Root>`,
		},
		{
			name:  "Mixed Content 2",
			input: `<Root><Child>A</Child> Spacer <Child>B</Child></Root>`,
		},

		// 5. Complex Cases
		{
			name:  "Deeply Nested With Attributes",
			input: `<Root id="root"><Child type="container"><SubChild extra="leaf">Data</SubChild></Child></Root>`,
		},
		{
			name:  "Siblings with Attributes",
			input: `<Root><Leaf id="1">Leaf1</Leaf><Leaf id="2">Leaf2</Leaf></Root>`,
		},
		{
			name:  "Many Unregistered Attributes",
			input: `<Root attr1="val1" attr2="val2" attr3="val3"><Child>Content</Child></Root>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Ground Truth Generation
			expectedStruct, err := parseXMLToElement(tt.input)
			if err != nil {
				t.Fatalf("Ground truth parser failed: %v", err)
			}
			// Debug:
			// t.Logf("Expected: %s", expectedStruct.String())

			// 2. Tokenize
			res, err := tokenizer.Tokenize(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Tokenization failed: %v", err)
			}

			// 3. Decode
			actualStruct, err := tokenizer.DecodeXML(res.Tokens)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			// 4. Verification
			elementsMatch(t, expectedStruct, actualStruct)
		})
	}
}
