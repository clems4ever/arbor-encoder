package main

import (
	"fmt"
	"os"

	"github.com/clems4ever/structured-encoder/tokenizer"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	tokenizer, err := tokenizer.NewTokenizer("examples/vocab.json")
	if err != nil {
		panic(err)
	}

	res, err := tokenizer.Tokenize(f)
	if err != nil {
		fmt.Printf("Error tokenizing: %v\n", err)
	}
	fmt.Printf("Tokens: %v\n", res.Tokens)
	fmt.Printf("Paths: %v\n", res.Paths)

	// Example of converting paths to a padded static tensor
	paddedPaths, maxDepth := res.GetPaddedPaths(0, -1)
	fmt.Printf("Max Depth: %d\n", maxDepth)
	fmt.Printf("Padded Paths (first 5 flattened): %v\n", paddedPaths[:5*maxDepth])

	decoded := tokenizer.Decode(res.Tokens)
	fmt.Printf("Decoded: %s\n", decoded)
}
