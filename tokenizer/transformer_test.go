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
		"@class":      3,
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
		"@class":   3,
		TokenEmpty: 88,
	}

	tr := NewTransformer(vocab)
	root, err := tr.Transform(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Element.String() produces <Tag></Tag> for now.
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
		TokenKey:         12,
		TokenKeyEnd:      13,
		TokenValue:       14,
		TokenValueEnd:    15,
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
