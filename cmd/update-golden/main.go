package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/clems4ever/structured-encoder/tokenizer"
)

func main() {
	// Paths are relative to the repository root
	inputFile := "tokenizer/testdata/example.html"
	outputFile := "tokenizer/testdata/example_golden.xml"

	// Verify we are in the right directory or finding the file
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Fatalf("Input file not found: %s. Please run this command from the repository root.", inputFile)
	}

	fmt.Printf("Reading %s...\n", inputFile)
	inputBytes, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	fmt.Println("Converting HTML to XML...")
	converted, err := tokenizer.ConvertHTMLToXML(strings.NewReader(string(inputBytes)))
	if err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}

	fmt.Printf("Writing to %s...\n", outputFile)
	if err := os.WriteFile(outputFile, []byte(converted), 0644); err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	fmt.Println("Done. Golden file updated.")
}
