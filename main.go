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
	fmt.Printf("Tokens (%d): %v\n", len(res.Tokens), res.Tokens)
	fmt.Printf("PaddedPaths (%d): %v\n", len(res.PaddedPaths), res.PaddedPaths)

	decoded := tokenizer.Decode(res.Tokens)
	fmt.Printf("Decoded: %s\n", decoded)
}
