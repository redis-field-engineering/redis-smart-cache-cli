package ConfirmationDialog

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"rsccli/RedisCommon"
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
		case tea.KeyEsc.String(), tea.KeyCtrlC.String(), "q":
			return m, tea.Quit
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

	body.WriteString("Would you like to commit the following Rules to Redis?\n")
	for _, r := range m.pendingRules {
		body.WriteString(r.GetJson() + "\n")
	}
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
