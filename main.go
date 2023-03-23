package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"rsccli/mainMenu"
)

func main() {
	p := tea.NewProgram(mainMenu.InitialModel())
	if res, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	} else {
		fmt.Println(res.(mainMenu.Model).Choice)
	}

}
