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

	tokenizer, err := tokenizer.NewTokenizer("vocab.json")
	if err != nil {
		panic(err)
	}

	tokens, depths, err := tokenizer.Tokenize(f)
	if err != nil {
		fmt.Printf("Error tokenizing: %v\n", err)
	}
	fmt.Printf("Tokens: %v\n", tokens)
	fmt.Printf("Depths: %v\n", depths)
}
