package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hieuhm2/lazyapi/internal/app"
	"github.com/hieuhm2/lazyapi/internal/storage"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "import":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: lazyapi import <file.json>")
				os.Exit(1)
			}
			runImport(os.Args[2])
			return
		case "help", "--help", "-h":
			printHelp()
			return
		case "version", "--version", "-v":
			fmt.Println("lazyapi dev")
			return
		}
	}

	a := app.New()
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runImport(filePath string) {
	fmt.Printf("Importing %s ...\n", filePath)

	collections, err := storage.ImportOpenAPI(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import error: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}

	existing, _ := store.LoadCollections()

	// Skip importing collections whose name already exists
	existingNames := map[string]bool{}
	for _, c := range existing {
		existingNames[c.Name] = true
	}
	var fresh []storage.Collection
	var skipped int
	for _, c := range collections {
		if existingNames[c.Name] {
			skipped++
		} else {
			fresh = append(fresh, c)
		}
	}

	merged := append(existing, fresh...)
	if err := store.SaveCollections(merged); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		os.Exit(1)
	}

	totalRequests := 0
	for _, c := range fresh {
		totalRequests += len(c.Requests)
	}

	fmt.Printf("✓  Imported %d collection(s) · %d request(s)\n", len(fresh), totalRequests)
	if skipped > 0 {
		fmt.Printf("   Skipped %d collection(s) — name already exists\n", skipped)
	}
	fmt.Printf("   Saved to ~/.config/lazyapi/collections.json\n")
	fmt.Printf("   Run 'lazyapi' to open.\n")
}

func printHelp() {
	fmt.Print(`lazyapi — lazygit-style TUI REST client

Usage:
  lazyapi                        start the TUI
  lazyapi import <file.json>     import OpenAPI 3.x / Swagger 2.x JSON

Keybindings (inside TUI):
  Tab / Shift+Tab / 1-4          switch panels
  h j k l                        navigate
  r / Enter                      execute request
  n                              new collection or request
  d                              delete
  e                              rename / edit body in $EDITOR
  m                              cycle HTTP method
  t / T                          next / prev tab
  i                              insert mode (edit URL)
  /                              search
  E                              cycle environment
  ?                              help
  q                              quit
`)
}
