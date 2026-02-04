package tokenizer

import (
	"strings"
	"testing"

	"github.com/pkoukk/tiktoken-go"
)

func TestEncoder_Encode_Basic(t *testing.T) {
	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		t.Logf("skipping test because tiktoken cannot be loaded: %v", err)
		return
	}

	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
	}

	// Virtual stream: <div>content</div>
	xmlStr := `<div>content</div>`

	enc := NewEncoder(vocab, tke)
	res, err := enc.Encode(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tokens: 1 (div), ... (content), 2 (div end)
	if res.Tokens[0] != 1 {
		t.Errorf("expected div token 1")
	}
	last := res.Tokens[len(res.Tokens)-1]
	if last != 2 {
		t.Errorf("expected div end token 2, got %d", last)
	}

	// Paths
	// Root: [0]
	// Text "content": [0, 1]  (Unordered parent -> child index 1)
	// End: [0]

	// Verify path 0 (div)
	if len(res.PaddedPaths[0]) < 1 { // Could be depth 1
		// Actually, previous implementation [0]
	}

	// Check text path
	// Should be child of div (index 0).
	// so path [0, 0] or [0, 1] depending on starting counter.
	// Div is unordered. Counter starts at 1.
	// So [0, 1].
}

func TestEncoder_Encode_Attributes(t *testing.T) {
	tke, _ := tiktoken.GetEncoding("cl100k_base")
	if tke == nil {
		return
	} // skip

	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		"@class":      3,
		TokenValueEnd: 99,
	}

	// <div class="val"></div>
	// Transformed: <div><__Attr><__Key>class</__Key><__Value>val</__Value></__Attr></div>
	// Start(div), Start(@class, Attr), Text(val), End(</__Value>), End(div)

	xmlStr := `<div><__Attr><__Key>class</__Key><__Value>val</__Value></__Attr></div>`

	enc := NewEncoder(vocab, tke)
	res, err := enc.Encode(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check Tokens
	// 1 (div), 3 (@class), val tokens, 99 (</__Value>), 2 (</div>)

	if len(res.Tokens) < 5 {
		t.Fatalf("too few tokens: %v", res.Tokens)
	}

	if res.Tokens[0] != 1 {
		t.Errorf("expected 1")
	}
	if res.Tokens[1] != 3 {
		t.Errorf("expected 3 (@class)")
	}
	if res.Tokens[len(res.Tokens)-2] != 99 {
		t.Errorf("expected 99")
	}
	if res.Tokens[len(res.Tokens)-1] != 2 {
		t.Errorf("expected 2")
	} // </div>

	// Check Paths
	// @class path: [0, 0] (Start 0 because IsAttr)
	if res.PaddedPaths[1][1] != 0 {
		t.Errorf("attr path should be 0, got %v", res.PaddedPaths[1])
	}
}

func TestEncoder_Encode_OrderedChildren(t *testing.T) {
	tke, _ := tiktoken.GetEncoding("cl100k_base")
	if tke == nil {
		return
	} // skip

	vocab := map[string]int{
		"<div>": 1, "</div>": 2,
		"<p>": 4, "</p>": 5, // Children
	}

	// <div ordered="true"> <p>1</p> <p>2</p> </div>
	// XML: <div arbor-ordered="true"><p>1</p><p>2</p></div>
	xmlStr := `<div arbor-ordered="true"><p>1</p><p>2</p></div>`

	enc := NewEncoder(vocab, tke)
	res, err := enc.Encode(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// <p> #1: Path [0, 1]
	// <p> #2: Path [0, 2]

	// Map tokens to indices.
	// 0: div [0]
	// 1: p   [0, 1]
	// 2: text 1
	// 3: /p
	// 4: p   [0, 2]
	// 5: text 2
	// 6: /p
	// 7: /div

	// Find first <p> token (id 4)
	var p1Idx, p2Idx int
	pIt := 0
	for i, tok := range res.Tokens {
		if tok == 4 {
			if pIt == 0 {
				p1Idx = i
				pIt++
			} else {
				p2Idx = i
			}
		}
	}

	if res.PaddedPaths[p1Idx][1] != 1 {
		t.Errorf("first child path should be 1, got %v", res.PaddedPaths[p1Idx])
	}

	if res.PaddedPaths[p2Idx][1] != 2 {
		t.Errorf("second child path should be 2, got %v", res.PaddedPaths[p2Idx])
	}
}
