package utils

import (
	"log"
	"os/exec"
	"runtime"
)

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
