package main

import (
	"fmt"
	"io"
	"os"

	"github.com/cehbz/tidalist/intent"
	"github.com/spf13/cobra"
)

func newLintIntentCmd() *cobra.Command {
	var write bool
	cmd := &cobra.Command{
		Use:   "lint-intent [file]",
		Short: "Validate and canonicalize an intent markdown file",
		Long:  "Reads intent markdown from a file (or stdin with '-'), validates it against the closed vocabulary, and writes the canonical form to stdout. Diagnostics go to stderr; a nonzero exit means validation errors.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var src []byte
			var path string
			if len(args) == 1 && args[0] != "-" {
				path = args[0]
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				src = b
			} else {
				if write {
					return fmt.Errorf("--write requires a file path, not stdin")
				}
				b, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				src = b
			}

			doc, ds := intent.Parse(src)
			ds = append(ds, intent.Validate(&doc)...)
			canon := intent.Canonical(doc)

			for _, d := range ds {
				fmt.Fprintln(cmd.ErrOrStderr(), d.String())
			}
			fmt.Fprintln(cmd.ErrOrStderr(), intent.Summary(doc))

			if intent.HasError(ds) {
				return fmt.Errorf("intent validation failed")
			}
			if write {
				return os.WriteFile(path, canon, 0o644)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(canon))
			return nil
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "rewrite the file in place with the canonical form")
	return cmd
}
