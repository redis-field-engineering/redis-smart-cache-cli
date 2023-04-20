package RuleTtlView

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"smart-cache-cli/RedisCommon"
	"smart-cache-cli/util"
)

type TableTtlMsg struct {
	Ttl string
}

type Model struct {
	textInput   textinput.Model
	table       *RedisCommon.Table
	pendingTtl  string
	parentModel *tea.Model
	err         string
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type.String() {
		case tea.KeyCtrlB.String(), tea.KeyEsc.String():
			return *m.parentModel, cmd
		case tea.KeyCtrlC.String():
			*m.parentModel, _ = (*m.parentModel).Update(msg)
			return *m.parentModel, tea.Quit
		case tea.KeyEnter.String():
			err := util.ValidateTimeout(m.textInput.Value())
			if err != nil {
				m.err = "\n" + err.Error()
			} else {
				*m.parentModel, cmd = (*m.parentModel).Update(TableTtlMsg{Ttl: m.textInput.Value()})
				return *m.parentModel, cmd
			}
		}

	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) View() string {
	return fmt.Sprintf("%s\n\nPress ctrl+b or escape to return to the previous screen\nInput TTL in the form of a duration e.g. 1h, 300s, 5m:\n%s%s", m.table.Formatted(), m.textInput.View(), m.err)
}

func New(table *RedisCommon.Table, parentModel tea.Model) Model {
	ti := textinput.New()
	ti.Placeholder = "30m"
	ti.Focus()
	ti.CharLimit = 30
	ti.Width = 30
	return Model{
		textInput:   ti,
		pendingTtl:  "",
		parentModel: &parentModel,
		table:       table,
	}
}
