package queryTtlView

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"rsccli/RedisCommon"
)

func (m Model) Init() tea.Cmd {
	return nil
}

type SetPendingTtlMsg struct {
	Ttl string
}

type Model struct {
	textInput   textinput.Model
	query       *RedisCommon.Query
	pendingTtl  string
	parentModel *tea.Model
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			//m.parentModel.UpdateCurrentTtl(m.textInput.Value())
			(*m.parentModel).Update(SetPendingTtlMsg{Ttl: m.textInput.Value()})
			return *m.parentModel, cmd
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return fmt.Sprintf("%s\n\nInput TTL in the form of a duration:\n%s", m.query.Formatted(), m.textInput.View())
}

func New(query *RedisCommon.Query, pm tea.Model) Model {
	ti := textinput.New()
	ti.Placeholder = "30m"
	ti.Focus()
	ti.CharLimit = 30
	ti.Width = 30

	return Model{
		textInput:   ti,
		pendingTtl:  "",
		parentModel: &pm,
		query:       query,
	}
}
