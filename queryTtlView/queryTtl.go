package queryTtlView

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"smart-cache-cli/RedisCommon"
	"smart-cache-cli/util"
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
	err         string
	width       int
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.textInput, _ = m.textInput.Update(msg)
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlB, tea.KeyEsc:
			return *m.parentModel, cmd
		case tea.KeyCtrlC:
			*m.parentModel, _ = (*m.parentModel).Update(msg)
			return *m.parentModel, tea.Quit
		case tea.KeyEnter:
			err := util.ValidateTimeout(m.textInput.Value())
			if err != nil {
				m.err = "\n" + err.Error()
			} else {
				(*m.parentModel).Update(SetPendingTtlMsg{Ttl: m.textInput.Value()})
				return *m.parentModel, cmd
			}
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {

	return fmt.Sprintf("%s\n\nPress ctrl+b or escape to return to the previous screen\nInput TTL in the form of a duration e.g. 1h, 300s, 5m:\n%s%s", m.query.Formatted(m.width), m.textInput.View(), m.err)
}

func New(query *RedisCommon.Query, pm tea.Model, width int) Model {
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
		width:       width,
	}
}
