package main

import (
	"github.com/spf13/cobra"

	"github.com/channyeintun/nami/internal/debuglog"
)

func newDebugViewCommand() *cobra.Command {
	options := debuglog.MonitorOptions{}
	cmd := &cobra.Command{
		Use:   "debug-view",
		Short: "Tail a structured debug log with a live monitor view",
		RunE: func(cmd *cobra.Command, args []string) error {
			return debuglog.RunMonitor(options)
		},
	}
	cmd.Flags().StringVar(&options.FilePath, "file", "", "Path to the session debug log")
	cmd.Flags().StringVar(&options.Level, "level", "", "Filter by log level")
	cmd.Flags().StringVar(&options.Component, "component", "", "Filter by component")
	cmd.Flags().StringVar(&options.Event, "event", "", "Filter by event name")
	cmd.Flags().BoolVar(&options.Raw, "raw", false, "Print raw JSONL instead of the formatted monitor view")
	cmd.Flags().IntVar(&options.Lines, "lines", 40, "Number of existing lines to print before following new events")
	return cmd
}
