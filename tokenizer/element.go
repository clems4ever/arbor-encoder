package tokenizer

import (
	"encoding/xml"
	"io"
	"strings"
)

// Element represents an XML node structure
type Element struct {
	Name       string
	Attributes []xml.Attr
	Children   []interface{} // *Element or string (CharData)
}

// String serializes the Element back to an XML string
func (e *Element) String() string {
	var sb strings.Builder
	e.writeTo(&sb)
	return sb.String()
}

func (e *Element) writeTo(sb *strings.Builder) {
	sb.WriteString("<" + e.Name)
	for _, attr := range e.Attributes {
		sb.WriteString(" " + attr.Name.Local + `="`)
		xml.EscapeText(sb, []byte(attr.Value))
		sb.WriteString(`"`)
	}
	// Check for self-closing if no children? 
    // The previous implementation of String() in decoder.go was:
    /*
	sb.WriteString(">")
	for _, child := range e.Children {
		switch c := child.(type) {
		case *Element:
			sb.WriteString(c.String())
		case string:
			sb.WriteString(c)
		}
	}
	sb.WriteString("</" + e.Name + ">")
    */
    // I should probably keep it compatible or improve it.
    
    if len(e.Children) == 0 {
         // Maybe self closing? Standard XML supports it. 
         // But let's stick to explicitly open/close to avoid issues unless empty.
         // <__Empty/> handling might be special.
    }

	sb.WriteString(">")
	for _, child := range e.Children {
		switch c := child.(type) {
		case *Element:
			c.writeTo(sb) // Recursive
		case string:
			xml.EscapeText(sb, []byte(c))
		}
	}
	sb.WriteString("</" + e.Name + ">")
}

func (e *Element) PrettyPrint(w io.Writer, depth int) {
	indent := strings.Repeat("  ", depth)

	// Determine if we should print inline (simple content) or block (complex content)
	isComplex := false
	for _, c := range e.Children {
		if _, ok := c.(*Element); ok {
			isComplex = true
			break
		}
	}

	io.WriteString(w, indent)
	io.WriteString(w, "<"+e.Name)
	for _, attr := range e.Attributes {
		io.WriteString(w, " "+attr.Name.Local+`="`)
		xml.EscapeText(w, []byte(attr.Value))
		io.WriteString(w, `"`)
	}

	if len(e.Children) == 0 {
		io.WriteString(w, " />\n")
		return
	}

	io.WriteString(w, ">")

	if isComplex {
		io.WriteString(w, "\n")
		for _, c := range e.Children {
			switch child := c.(type) {
			case *Element:
				child.PrettyPrint(w, depth+1)
			case string:
				trimmed := strings.TrimSpace(child)
				if trimmed != "" {
					io.WriteString(w, strings.Repeat("  ", depth+1))
					xml.EscapeText(w, []byte(trimmed))
					io.WriteString(w, "\n")
				}
			}
		}
		io.WriteString(w, indent)
	} else {
		// All children are strings
		for _, c := range e.Children {
			if str, ok := c.(string); ok {
				xml.EscapeText(w, []byte(str))
			}
		}
	}

	io.WriteString(w, "</"+e.Name+">\n")
}
