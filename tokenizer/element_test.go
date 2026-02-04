package tokenizer

import (
	"bytes"
	"encoding/xml"
	"testing"
)

func TestElement_String(t *testing.T) {
	el := &Element{
		Name: "Div",
		Attributes: []xml.Attr{
			{Name: xml.Name{Local: "class"}, Value: "container"},
		},
		Children: []interface{}{
			"Text",
			&Element{Name: "Span", Children: []interface{}{"More Text"}},
		},
	}

	expected := `<Div class="container">Text<Span>More Text</Span></Div>`
	if got := el.String(); got != expected {
		t.Errorf("String() mismatch:\nExpected: %s\nGot:      %s", expected, got)
	}
}

func TestElement_PrettyPrint(t *testing.T) {
	el := &Element{
		Name: "Div",
		Attributes: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: "main"},
		},
		Children: []interface{}{
			&Element{Name: "P", Children: []interface{}{"Hello"}},
		},
	}

	var buf bytes.Buffer
	el.PrettyPrint(&buf, 0)

	expected := `<Div id="main">
  <P>Hello</P>
</Div>
`
	if got := buf.String(); got != expected {
		t.Errorf("PrettyPrint() mismatch:\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}
