package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "arbor-encoder",
	Short: "A specialized XML tokenizer for ML applications",
	Long: `Arbor Encoder converts XML documents into a sequence of tokens 
accompanied by structural path embeddings, allowing models to understand 
the hierarchical position of each token.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {}
