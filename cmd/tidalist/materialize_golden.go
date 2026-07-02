package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cehbz/tidalist/curate"
	"github.com/spf13/cobra"
)

func newMaterializeGoldenCmd() *cobra.Command {
	var output, reportPath string
	cmd := &cobra.Command{
		Use:   "materialize-golden [selections.json]",
		Short: "Materialize LLM-chosen identities into the Golden Master + curate report",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if len(args) == 0 || args[0] == "-" {
				data, err = io.ReadAll(cmd.InOrStdin())
			} else {
				data, err = os.ReadFile(args[0])
			}
			if err != nil {
				return err
			}
			sel, err := curate.ParseSelections(data)
			if err != nil {
				return err
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			doc, rep, err := curate.Materialize(m, sel)
			if err != nil {
				return err
			}
			if reportPath != "" {
				b, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(reportPath, append(b, '\n'), 0o644); err != nil {
					return err
				}
			}
			if output != "" {
				b, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					return err
				}
				return os.WriteFile(output, append(b, '\n'), 0o644)
			}
			b, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write the golden JSON here (default: stdout)")
	cmd.Flags().StringVar(&reportPath, "report", "", "write the curate report JSON here")
	return cmd
}
