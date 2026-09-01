package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
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

func OpenUrl(url string) {
	var err error

	switch runtime.GOOS {
	case "darwin": // macOS
		err = exec.Command("open", url).Start()
	case "windows": // Windows
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default: // Linux and other Unix-like systems
		err = exec.Command("xdg-open", url).Start()
	}

	if err != nil {
		log.Fatal("Failed to open browser: ", err)
	}
}
