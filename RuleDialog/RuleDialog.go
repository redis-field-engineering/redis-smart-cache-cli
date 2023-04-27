package RuleDialog

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
	"io"
	"smart-cache-cli/ConfirmationDialog"
	"smart-cache-cli/RedisCommon"
	"smart-cache-cli/util"
	"strings"
)

type item struct{ ruleType RedisCommon.RuleType }

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s", i.ruleType)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + s[0])
		}
	}

	fmt.Fprint(w, fn(str))
}

var (
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	itemStyle           = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle   = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
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
	typeSelectorList list.Model
	focusIndex       int
	error            string
	parentModel      tea.Model
	rdb              *redis.Client
	confirm          bool
	isNew            bool
	applicationName  string
	ruleType         RedisCommon.RuleType
	ttl              string
	match            string
	textInput        textinput.Model
	wasPreset        bool
}

func New(parentModel tea.Model, rdb *redis.Client, rule *RedisCommon.Rule, confirm bool, applicationName string, ruleType RedisCommon.RuleType) Model {
	items := []list.Item{
		item{RedisCommon.Tables},
		item{RedisCommon.TablesAll},
		item{RedisCommon.TablesAny},
		item{RedisCommon.Regex},
		item{RedisCommon.All},
	}
	typeSelectList := list.New(items, itemDelegate{}, 50, 15)
	typeSelectList.Title = "Select a Rule Type"

	ti := textinput.New()

	ti.Focus()
	ti.CharLimit = 30
	ti.Width = 30
	wasPreset := ruleType != RedisCommon.Unknown

	m := Model{
		parentModel:      parentModel,
		rdb:              rdb,
		confirm:          confirm,
		isNew:            rule == nil,
		applicationName:  applicationName,
		typeSelectorList: typeSelectList,
		ruleType:         ruleType,
		textInput:        ti,
		ttl:              "",
		wasPreset:        wasPreset,
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) GetRuleFromModel() (*RedisCommon.Rule, error) {

	rule := RedisCommon.Rule{
		Ttl: m.ttl,
	}

	switch m.ruleType {
	case RedisCommon.Tables:
		rule.Tables = strings.Split(m.match, ",")
	case RedisCommon.TablesAny:
		rule.TablesAny = strings.Split(m.match, ",")
	case RedisCommon.TablesAll:
		rule.TablesAll = strings.Split(m.match, ",")
	case RedisCommon.QueryIds:
		rule.QueryIds = strings.Split(m.match, ",")
	case RedisCommon.Regex:
		rule.Regex = &m.match
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
		if m.ruleType == RedisCommon.Unknown {
			m.typeSelectorList, _ = m.typeSelectorList.Update(msg)
		}
		switch msg.String() {
		case tea.KeyCtrlC.String():
			m.parentModel, _ = m.parentModel.Update(msg)
			return m.parentModel, tea.Quit
		case tea.KeyCtrlB.String(), tea.KeyEsc.String():
			m.textInput.SetValue("")
			if util.ValidateTimeout(m.ttl) == nil {
				m.ttl = ""
				return m, nil
			}
			if m.match != "" {
				m.match = ""
				return m, nil
			} else if m.ruleType != RedisCommon.Unknown && !m.wasPreset {
				m.ruleType = RedisCommon.Unknown
				return m, nil
			}

			m.parentModel, _ = m.parentModel.Update(ConfirmationDialog.ConfirmationMessage{ConfirmedUpdate: true})
			return m.parentModel, nil
		case tea.KeyTab.String(), tea.KeyShiftTab.String(), tea.KeyEnter.String(), tea.KeyUp.String(), tea.KeyDown.String():
			s := msg.String()

			if s == "enter" {
				if m.ruleType == RedisCommon.Unknown {
					i, _ := m.typeSelectorList.SelectedItem().(item)
					m.ruleType = i.ruleType
					return m, nil
				} else if m.match == "" && m.ruleType != RedisCommon.All {
					if m.textInput.Value() != "" {
						m.match = m.textInput.Value()
						m.textInput.SetValue("")
						return m, nil
					}
				} else if util.ValidateTimeout(m.ttl) != nil {
					candidateTtl := m.textInput.Value()
					if util.ValidateTimeout(candidateTtl) == nil {
						m.ttl = candidateTtl
						m.textInput.SetValue("")
						m.textInput.Placeholder = ""
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
				}
			}
		}
		if m.ruleType != RedisCommon.Unknown {
			m.textInput, _ = m.textInput.Update(msg)
		}

	}
	return m, nil

}

func (m Model) ruleSoFar() string {
	var b strings.Builder
	if m.ruleType != RedisCommon.Unknown {
		b.WriteString(fmt.Sprintf("Rule Type: %s\n", m.ruleType))
	}
	if m.match != "" {
		b.WriteString(fmt.Sprintf("Match Against: %s\n", m.match))
	}

	return b.String()
}

func (m Model) View() string {
	var b strings.Builder

	if m.ruleType == RedisCommon.Unknown {
		b.WriteString("select a rule type, press ctrl+b to return to the previous screen\n")
		b.WriteString(m.typeSelectorList.View())
		return b.String()
	} else if m.match == "" && m.ruleType != RedisCommon.All {
		b.WriteString(m.ruleSoFar())
		if m.ruleType == RedisCommon.Regex {
			b.WriteString("Input a regular expression to match against:")
		} else if m.ruleType == RedisCommon.QueryIds {
			b.WriteString("Input a comma delimited list of Query Ids to match against:")
		} else {
			b.WriteString("Input a comma delimited list of tables to match against:")
		}
		b.WriteString(m.textInput.View())
	} else {
		b.WriteString(m.ruleSoFar())
		b.WriteString("Input TTL in the form of a duration e.g. 1h, 300s, 5m: ")
		b.WriteString(m.textInput.View())
	}

	return b.String()
}
