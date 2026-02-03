package cmd

import (
	"fmt"
	"os"

	"github.com/clems4ever/structured-encoder/tokenizer"
	"github.com/spf13/cobra"
)

var vocabPath string

// tokenizeCmd represents the tokenize command
var tokenizeCmd = &cobra.Command{
	Use:   "tokenize [xml_file]",
	Short: "Tokenize an XML file",
	Long:  `Tokenize an XML file and print the tokens and path embeddings.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		tok, err := tokenizer.NewTokenizer(vocabPath)
		if err != nil {
			fmt.Printf("Error creating tokenizer: %v\n", err)
			os.Exit(1)
		}

		res, err := tok.Tokenize(f)
		if err != nil {
			fmt.Printf("Error tokenizing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tokens (%d): %v\n", len(res.Tokens), res.Tokens)
		fmt.Printf("PaddedPaths (%d): %v\n", len(res.PaddedPaths), res.PaddedPaths)

		decoded := tok.Decode(res.Tokens)
		fmt.Printf("Decoded: %s\n", decoded)
	},
}

func init() {
	rootCmd.AddCommand(tokenizeCmd)

	tokenizeCmd.Flags().StringVarP(&vocabPath, "vocab", "v", "examples/vocab.json", "Path to vocabulary file")
}
