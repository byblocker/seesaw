package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const examplesDir = "./examples"

var initCmd = &cobra.Command{
	Use:   "init <portfolio>",
	Short: "Create the user config and a portfolio plan from the examples",
	Args:  cobra.ExactArgs(1),
	RunE:  initFiles,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func initFiles(cmd *cobra.Command, args []string) error {
	name := args[0]
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("portfolio name must not be empty")
	}

	planPath := filepath.Join(portfolioDir, name+".yaml")
	if _, err := os.Stat(planPath); err == nil {
		return fmt.Errorf("portfolio plan %s already exists", planPath)
	}

	out := cmd.OutOrStdout()
	created, err := copyExample("user.yaml", userConfigPath)
	if err != nil {
		return err
	}

	if created {
		fmt.Fprintf(out, "Created %s\n", userConfigPath)
	} else {
		fmt.Fprintf(out, "Kept existing %s\n", userConfigPath)
	}

	if _, err := copyExample("portfolio.yaml", planPath); err != nil {
		return err
	}
	fmt.Fprintf(out, "Created %s\n", planPath)
	return nil
}

func copyExample(example, path string) (bool, error) {
	source, err := os.Open(filepath.Join(examplesDir, example))
	if err != nil {
		return false, fmt.Errorf("opening example; run init from the repository root: %w", err)
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}

	target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, target.Close()
}
