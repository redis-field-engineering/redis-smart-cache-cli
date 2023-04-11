package mainMenu

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/redis/go-redis/v9"
	"io"
	"smart-cache-cli/ConfirmationDialog"
	"smart-cache-cli/RuleDialog"
	"smart-cache-cli/RuleList"
	"smart-cache-cli/queryList"
)

const listHeight = 14

type Model struct {
	list     list.Model
	message  string
	Choice   string
	quitting bool
	rdb      *redis.Client
}

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
)

type item string

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

	str := fmt.Sprintf("%s", i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + s[0])
		}
	}

	fmt.Fprint(w, fn(str))
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case ConfirmationDialog.ConfirmationMessage:
		m.Choice = ""
		m.message = msg.Message
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter", " ":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.Choice = string(i)
				if string(i) == "List Queries" {
					return queryList.InitialModel(m, m.rdb), nil
				} else if string(i) == "Create Rule" {
					return RuleDialog.New(m, m.rdb, nil, true), nil
				} else if string(i) == "Rule List" {
					return RuleList.New(m, m.rdb), nil
				}
			}
			return m, tea.Quit
		}

	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.Choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("Lets %s", m.Choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Goodbye")
	}
	return "\n" + m.list.View() + "\n" + m.message
}

func InitialModel(rdb *redis.Client) Model {
	items := []list.Item{
		item("List Queries"),
		item("Rule List"),
		item("Create Rule"),
	}

	const defaultWidth = 20

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Action"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	return Model{list: l, rdb: rdb}
}
