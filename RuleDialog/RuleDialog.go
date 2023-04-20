package RuleDialog

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
	"smart-cache-cli/ConfirmationDialog"
	"smart-cache-cli/RedisCommon"
	"strings"
)

var (
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	helpStyle           = blurredStyle.Copy()
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

type RuleMsg struct {
	Rule  RedisCommon.Rule
	IsNew bool
}

type Model struct {
	focusIndex      int
	inputs          []textinput.Model
	instructions    []string
	cursorMode      textinput.CursorMode
	error           string
	parentModel     tea.Model
	rdb             *redis.Client
	confirm         bool
	isNew           bool
	applicationName string
}

func New(parentModel tea.Model, rdb *redis.Client, rule *RedisCommon.Rule, confirm bool, applicationName string) Model {
	m := Model{
		inputs:          make([]textinput.Model, 6),
		instructions:    make([]string, 6),
		parentModel:     parentModel,
		rdb:             rdb,
		confirm:         confirm,
		isNew:           rule == nil,
		applicationName: applicationName,
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.CursorStyle = cursorStyle

		switch i {
		case 0:
			m.instructions[i] = "Time To Live as Duration (e.g. 1h):							"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
			if rule != nil && rule.Ttl != "" {
				t.SetValue(rule.Ttl)
			} else {
				t.Placeholder = "TTL"
			}
		case 1:
			m.instructions[i] = "Match if exactly these tables appear in the SQL Query (comma delimited list):		"
			if rule != nil && rule.Tables != nil {
				t.SetValue(strings.Join(rule.Tables, ","))
			} else {
				t.Placeholder = "Tables"
			}
		case 2:
			m.instructions[i] = "Match if any of these tables appear in the SQL Query (comma delimited list):		"
			if rule != nil && rule.TablesAny != nil {
				t.SetValue(strings.Join(rule.TablesAny, ","))
			} else {
				t.Placeholder = "Tables Any"
			}
		case 3:
			m.instructions[i] = "Match if all of these tables appear in the SQL query (comma delimited list):		"
			if rule != nil && rule.TablesAll != nil {
				t.SetValue(strings.Join(rule.TablesAll, ","))
			} else {
				t.Placeholder = "Tables All"
			}
		case 4:
			m.instructions[i] = "Match if any of these query IDs show up in the SQL query (comma delimited list):	"
			if rule != nil && rule.QueryIds != nil {
				t.SetValue(strings.Join(rule.QueryIds, ","))
			} else {
				t.Placeholder = "Query Ids"
			}
		case 5:
			m.instructions[i] = "Match if the SQL query matches this Regex:						"
			if rule != nil && rule.Regex != nil {
				t.SetValue(*rule.Regex)
			} else {
				t.Placeholder = "Regex"
			}
		}

		m.inputs[i] = t
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) GetRuleFromModel() (*RedisCommon.Rule, error) {

	rule := RedisCommon.Rule{}

	if m.inputs[0].Value() != "" {
		rule.Ttl = m.inputs[0].Value()
	} else {
		return nil, errors.New("TTL missing")
	}

	if m.inputs[1].Value() != "Tables" && m.inputs[1].Value() != "" {
		rule.Tables = strings.Split(m.inputs[1].Value(), ",")
	}

	if m.inputs[2].Value() != "TablesAny" && m.inputs[2].Value() != "" {
		rule.TablesAny = strings.Split(m.inputs[2].Value(), ",")
	}

	if m.inputs[3].Value() != "TablesAll" && m.inputs[3].Value() != "" {
		rule.TablesAll = strings.Split(m.inputs[3].Value(), ",")
	}

	if m.inputs[4].Value() != "QueryIds" && m.inputs[4].Value() != "" {
		rule.QueryIds = strings.Split(m.inputs[4].Value(), ",")
	}

	if m.inputs[5].Value() != "Regex" && m.inputs[5].Value() != "" {
		v := m.inputs[5].Value()
		rule.Regex = &v
	}

	return &rule, nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ConfirmationDialog.ConfirmationMessage:
		m.parentModel, _ = m.parentModel.Update(msg)
		rule, _ := m.GetRuleFromModel()
		_, err := RedisCommon.CommitNewRules(m.rdb, []RedisCommon.Rule{*rule}, m.applicationName)
		if err != nil {
			confMsg := ConfirmationDialog.ConfirmationMessage{
				Message: "Failed to update Redis",
			}

			m.parentModel, _ = m.parentModel.Update(confMsg)
		}
		return m.parentModel, nil
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyCtrlC.String(), tea.KeyEsc.String():
			return m.parentModel, tea.Quit
		case tea.KeyCtrlB.String():
			m.parentModel, _ = m.parentModel.Update(ConfirmationDialog.ConfirmationMessage{ConfirmedUpdate: true})
			return m.parentModel, nil
		case tea.KeyTab.String(), tea.KeyShiftTab.String(), tea.KeyEnter.String(), tea.KeyUp.String(), tea.KeyDown.String():
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs) {
				rule, err := m.GetRuleFromModel()
				if err != nil {
					m.error = err.Error()
					return m, nil
				}

				if !m.confirm {
					respMsg := RuleMsg{
						Rule:  *rule,
						IsNew: m.isNew,
					}

					m.parentModel, _ = m.parentModel.Update(respMsg)
					return m.parentModel, nil
				}

				ruleMap := make(map[string]RedisCommon.Rule)
				ruleMap[rule.Ttl] = *rule
				return ConfirmationDialog.New(m, ruleMap), nil
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}

				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			return m, tea.Batch(cmds...)
		}
	}

	cmd := m.updateInputs(msg)

	return m, cmd

}

func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("Enter your Rule details, press ctrl+b to return to the previous screen\n")

	for i := range m.inputs {
		b.WriteString(fmt.Sprintf("%s%s", m.instructions[i], m.inputs[i].View()))
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}

	fmt.Fprintf(&b, "\n%s\n%s\n\n", m.error, *button)

	return b.String()
}
