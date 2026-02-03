package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/clems4ever/structured-encoder/tokenizer"
)

func main() {
	// Paths are relative to the repository root
	inputs, err := filepath.Glob("tokenizer/testdata/*.html")
	if err != nil {
		log.Fatalf("Failed to glob files: %v", err)
	}

	for _, inputFile := range inputs {
		outputFile := strings.TrimSuffix(inputFile, ".html") + "_golden.xml"

		fmt.Printf("Processing %s -> %s\n", inputFile, outputFile)
		inputBytes, err := os.ReadFile(inputFile)
		if err != nil {
			log.Printf("Failed to read input file %s: %v", inputFile, err)
			continue
		}

		converted, err := tokenizer.ConvertHTMLToXML(strings.NewReader(string(inputBytes)))
		if err != nil {
			log.Printf("Conversion failed for %s: %v", inputFile, err)
			continue
		}

		if err := os.WriteFile(outputFile, []byte(converted), 0644); err != nil {
			log.Printf("Failed to write output file %s: %v", outputFile, err)
			continue
		}
	}

	fmt.Println("Done. Golden files updated.")
}
