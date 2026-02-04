package tokenizer

import (
	"encoding/xml"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/pkoukk/tiktoken-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDecoder_Coverage_EdgeCases(t *testing.T) {
	tk, err := tiktoken.GetEncoding("cl100k_base")
	require.NoError(t, err)

	vocab := map[string]int{
		"<root>":                 100,
		"</root>":                101,
		"<child>":                102,
		"</child>":               103,
		TokenUnregisteredAttr:    104,
		TokenUnregisteredAttrEnd: 105,
		TokenKey:                 106,
		TokenKeyEnd:              107,
		TokenValue:               108,
		TokenValueEnd:            109,
		"##attr":                 110,
		TokenEmpty:               111,
		"##attr2":                112,
	}

	vocabInv := make(map[int]string)
	for k, v := range vocab {
		vocabInv[v] = k
	}

	tokenizer := &Tokenizer{
		vocab:            vocab,
		vocabInv:         vocabInv,
		contentTokenizer: tk,
	}

	t.Run("Unexpected_End_Tag_At_Root", func(t *testing.T) {
		// Tokens: </root>
		tokens := []int{101}
		_, err := tokenizer.DecodeXML(tokens)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected end tag")
	})

	t.Run("UnregisteredAttr_End_Of_Stream", func(t *testing.T) {
		// Tokens: <root> <__UnregisteredAttr> <__Key> key (no end)
		// Assuming we can inject content tokens. Let's use words "key", "val".
		// key -> e.g. 500
		keyTok := tk.Encode("key", nil, nil)[0]

		tokens := []int{100, 104, 106, keyTok}
		// Code does: for i < len(tokens) ... if loop finishes, it breaks.
		// It appends whatever it got.

		el, err := tokenizer.DecodeXML(tokens)
		assert.NoError(t, err)
		require.NotNil(t, el)
		// Should have attribute key="key" value="" (since we entered Key state, wrote key, loop ended)
		// Wait, state starts at 0. <__Key> sets state=1. CharData writes to key.
		// Loop ends. Attribute appended.
		assert.Equal(t, "root", el.Name)
		assert.Len(t, el.Attributes, 1)
		assert.Equal(t, "key", el.Attributes[0].Name.Local)
		assert.Equal(t, "", el.Attributes[0].Value)
	})

	t.Run("RegisteredAttr_ExplicitEmpty", func(t *testing.T) {
		// Tokens: <root> ##attr <__Empty>
		tokens := []int{100, 110, 111, 101}
		el, err := tokenizer.DecodeXML(tokens)
		assert.NoError(t, err)
		require.NotNil(t, el)
		assert.Len(t, el.Attributes, 1)
		assert.Equal(t, "attr", el.Attributes[0].Name.Local)
		assert.Equal(t, "", el.Attributes[0].Value)
	})

	t.Run("RegisteredAttr_ImplicitEnd_By_Tag", func(t *testing.T) {
		// Tokens: <root> ##attr val <child> ...
		valTok := tk.Encode("val", nil, nil)[0]
		tokens := []int{100, 110, valTok, 102, 103, 101}
		el, err := tokenizer.DecodeXML(tokens)
		assert.NoError(t, err)
		require.NotNil(t, el)
		assert.Len(t, el.Attributes, 1)
		assert.Equal(t, "attr", el.Attributes[0].Name.Local)
		assert.Equal(t, "val", el.Attributes[0].Value)
		assert.Len(t, el.Children, 1)
		// <child>...
	})

	t.Run("RegisteredAttr_ImplicitEnd_By_NextAttr", func(t *testing.T) {
		// Tokens: <root> ##attr val1 ##attr2 val2
		// careful with tokenization of "val1" - might be split. Use "foo" and "bar"
		valTok1 := tk.Encode("foo", nil, nil)[0]
		valTok2 := tk.Encode("bar", nil, nil)[0]
		tokens := []int{100, 110, valTok1, 112, valTok2, 101}
		el, err := tokenizer.DecodeXML(tokens)
		assert.NoError(t, err)
		require.NotNil(t, el)
		assert.Len(t, el.Attributes, 2)
		assert.Equal(t, "attr", el.Attributes[0].Name.Local)
		assert.Equal(t, "foo", el.Attributes[0].Value)
		assert.Equal(t, "attr2", el.Attributes[1].Name.Local)
		assert.Equal(t, "bar", el.Attributes[1].Value)
	})

	t.Run("Skip_Special_Tokens", func(t *testing.T) {
		// Tokens: <root> <__ValueEnd> <__Key> content </root>
		// Special tokens appearing out of context should be skipped.
		tokens := []int{100, 109, 106, tk.Encode("content", nil, nil)[0], 101}
		el, err := tokenizer.DecodeXML(tokens)
		assert.NoError(t, err)
		require.NotNil(t, el)
		assert.Len(t, el.Children, 1)
		assert.Equal(t, "content", el.Children[0].(string))
	})

	t.Run("Empty_Tokens", func(t *testing.T) {
		el, err := tokenizer.DecodeXML([]int{})
		assert.NoError(t, err)
		assert.Nil(t, el)
	})
}
