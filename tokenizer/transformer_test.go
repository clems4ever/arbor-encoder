package tokenizer

import (
	"strings"
	"testing"
)

func TestTransformer_Transform_Basic(t *testing.T) {
	xmlStr := `<div>content</div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
	}

	tr := NewTransformer(vocab)
	xmlBytes, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div>content</div>`
	if string(xmlBytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(xmlBytes))
	}
}

func TestTransformer_Attributes_Registered(t *testing.T) {
	xmlStr := `<div class="foo"></div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		"@class":      3,
		TokenValueEnd: 99,
	}

	tr := NewTransformer(vocab)
	xmlBytes, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output: <div><__Attr name="class">foo</__Attr></div>
	// Or maybe the transformer escapes the quotes?
	// `name="class"` -> standard xml attribute.

	expected := `<div><__Attr name="class">foo</__Attr></div>`
	if string(xmlBytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(xmlBytes))
	}
}

func TestTransformer_Attributes_Registered_Empty(t *testing.T) {
	xmlStr := `<div class=""></div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		"@class":   3,
		TokenEmpty: 88,
	}

	tr := NewTransformer(vocab)
	xmlBytes, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output: <div><__Attr name="class"><__Empty /></__Attr></div>
	expected := `<div><__Attr name="class"><__Empty /></__Attr></div>`
	if string(xmlBytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(xmlBytes))
	}
}

func TestTransformer_Attributes_Unregistered(t *testing.T) {
	xmlStr := `<div unknown="val"></div>`
	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		TokenAttrPair:    10,
		TokenAttrPairEnd: 11,
		TokenKey:         12,
		TokenKeyEnd:      13,
		TokenValue:       14,
		TokenValueEnd:    15,
	}

	tr := NewTransformer(vocab)
	xmlBytes, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output (approximate):
	// <div>
	//   <__AttrPair>
	//      <__Key>unknown</__Key>
	//      <__Value>val</__Value>
	//   </__AttrPair>
	// </div>

	// We check for presence of substrings or exact construction logic.
	// Since we manually construct strings, we can predict exact output.
	// <__AttrPair><__Key>unknown</__Key><__Value>val</__Value></__AttrPair>

	expected := `<div><__AttrPair><__Key>unknown</__Key><__Value>val</__Value></__AttrPair></div>`
	if string(xmlBytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(xmlBytes))
	}
}

func TestTransformer_Ordered(t *testing.T) {
	xmlStr := `<div arbor-ordered="true"></div>`
	vocab := map[string]int{"<div>": 1, "</div>": 2}
	tr := NewTransformer(vocab)
	xmlBytes, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div arbor-ordered="true"></div>`
	if string(xmlBytes) != expected {
		t.Errorf("expected %s, got %s", expected, string(xmlBytes))
	}
}
