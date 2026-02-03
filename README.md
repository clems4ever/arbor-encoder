# Structured Encoder

![Build Status](https://github.com/clems4ever/structured-encoder/actions/workflows/test.yml/badge.svg)

This project implements a specialized XML tokenizer in Go designed for machine learning applications (e.g., Transformers). It converts XML documents into a sequence of tokens accompanied by structural path embeddings, allowing models to understand the hierarchical position of each token.

## Features

-   **Hybrid Tokenization**: Uses a custom XML parser for tags and `tiktoken` (cl100k_base) for text content.
-   **Structure-Awareness**: Generates a coordinate path (tree position) for every token, returned as a padded tensor.
-   **Order Invariance**: Supports the `ordered="false"` attribute on XML tags. Siblings within an unordered container share the same structural path index, allowing the model to treat them as permutation-invariant.
-   **Static Tensor Output**: Outputs `PaddedPaths` as a rectangular 2D matrix (batch-ready) suitable for concatenation with token embeddings.

## Installation

```bash
go get github.com/clems4ever/structured-encoder
```

## Usage

### Prepare Vocabulary

You need a JSON vocabulary file mapping XML tags to integer IDs. The content tokenizer uses OpenAI's `cl100k_base` encoding.

**vocab.json**:
```json
{
  "<Root>": 1001,
  "</Root>": 1002,
  "<Item>": 1003,
  "</Item>": 1004
}
```

### Tokenize a File

```go
package main

import (
	"fmt"
	"os"

	"github.com/clems4ever/structured-encoder/tokenizer"
)

func main() {
	// Initialize with your vocabulary
	tok, err := tokenizer.NewTokenizer("vocab.json")
	if err != nil {
		panic(err)
	}

	f, _ := os.Open("data.xml")
	defer f.Close()

	// Tokenize
	res, err := tok.Tokenize(f)
	if err != nil {
		panic(err)
	}

	// Access Results
	fmt.Printf("Tokens: %v\n", res.Tokens)
	// PaddedPaths is a [][]int matrix representing the tree coordinates of each token
	fmt.Printf("Paths: %v\n", res.PaddedPaths)
}
```

## Encoding Logic

### Path Coordinates
Every token is assigned a path vector representing its location in the XML tree.
- **Root**: `[0]`
- **Child of Root**: `[0, 0]`, `[0, 1]`, etc.

### Order Invariance
If an XML tag has `ordered="false"`, its children will not increment the sibling counter. This means all children will effectively have the same "position" index, signaling to the model that their relative order does not matter.

```xml
<List ordered="false">
  <Item>A</Item> <!-- Path: [0, 0, 0] -->
  <Item>B</Item> <!-- Path: [0, 0, 0] -->
</List>
```

## Integration with ML Models

The `PaddedPaths` output is designed to be fed into a model alongside the token IDs. A common strategy is:

1.  **Token Embedding**: Lookup `Tokens` in an embedding table.
2.  **Path Encoding**: Feed each row of `PaddedPaths` into a small MLP or encoder to get a path vector.
3.  **Combine**: Concatenate or sum the Token Embedding and Path Vector.
4.  **Transformer**: Pass the result to standard attention layers.
