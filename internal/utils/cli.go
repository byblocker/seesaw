package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
)

func AskForInput(prompt string) (string, error) {
	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(text), nil
}

func AskForSecret(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("credential setup requires an interactive terminal")
	}

	fmt.Fprint(os.Stderr, prompt)
	secret, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("failed to read credential: %w", err)
	}
	defer clear(secret)

	return strings.TrimSpace(string(secret)), nil
}

func OpenUrl(url string) error {
	prompt := fmt.Sprintf("Open in your browser:\n%s\nPress ENTER to continue: ", url)
	if _, err := AskForInput(prompt); err != nil {
		return err
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot open browser: %v", err)
	}
	return nil
}
