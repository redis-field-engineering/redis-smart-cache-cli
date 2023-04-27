package ConfirmationDialog

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"smart-cache-cli/RedisCommon"
	"strings"
)

func (m Model) Init() tea.Cmd {
	return nil
}

type ConfirmationMessage struct {
	Message         string
	ConfirmedUpdate bool
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case tea.KeyEsc.String(), "b":
			m.parentModel, _ = m.parentModel.Update(msg)
			return m.parentModel, nil
		case tea.KeyCtrlC.String(), "q":
			m.parentModel, _ = m.parentModel.Update(msg)
			return m.parentModel, tea.Quit
		case "y", "Y":
			if m.parentModel == nil {
				m.Confirmed = true
				return m, tea.Quit
			}
			m.parentModel, _ = m.parentModel.Update(ConfirmationMessage{
				Message:         "Rule Updates Committed to Redis.",
				ConfirmedUpdate: true,
			})
			return m.parentModel, cmd

		case "n", "N":
			if m.parentModel == nil {
				m.Confirmed = false
				return m, tea.Quit
			}
			m.parentModel.Update(ConfirmationMessage{
				ConfirmedUpdate: false,
			})
			return m.parentModel, cmd

		}
	}

	return m, cmd
}

func (m Model) View() string {
	body := strings.Builder{}
	noun := "rule"
	if len(m.pendingRules) > 1 {
		noun = "rules"
	}

	body.WriteString(fmt.Sprintf("Would you like to commit the following %s to Redis?\n", noun))
	body.WriteString("=============Rules To Commit==============\n")
	for _, r := range m.pendingRules {
		body.WriteString(r.Formatted())
		body.WriteString("\n")

	}

	body.WriteString("===========================================\n")
	body.WriteString("y/N")
	return body.String()
}

type Model struct {
	parentModel  tea.Model
	inputMode    textinput.Model
	pendingRules map[string]RedisCommon.Rule
	Confirmed    bool
}

func New(parentModel tea.Model, pendingRules map[string]RedisCommon.Rule) Model {
	ti := textinput.New()
	ti.Focus()
	return Model{
		parentModel:  parentModel,
		inputMode:    ti,
		pendingRules: pendingRules,
	}
}
