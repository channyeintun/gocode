package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/channyeintun/nami/internal/timing"
)

func newTimingSummaryCommand() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "timing-summary",
		Short: "Summarize session timing and compaction traces from timings.ndjson",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}
			summary, err := timing.SummarizeFile(filePath)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), summary.Render())
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to timings.ndjson")
	return cmd
}
