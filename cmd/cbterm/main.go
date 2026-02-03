package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/calobozan/cbterm/internal/app"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	shell := flag.String("shell", "", "Shell to run (default: $SHELL or /bin/zsh)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("term %s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Determine shell
	shellPath := *shell
	if shellPath == "" {
		shellPath = os.Getenv("SHELL")
		if shellPath == "" {
			shellPath = "/bin/zsh"
		}
	}

	// Create and run the terminal app
	termApp, err := app.New(app.Config{
		Shell: shellPath,
	})
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	if err := termApp.Run(); err != nil {
		log.Fatalf("App error: %v", err)
	}
}
