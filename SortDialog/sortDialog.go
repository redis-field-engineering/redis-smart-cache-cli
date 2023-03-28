package SortDialog

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"io"
)

type Direction string

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + s[0])
		}
	}

	fmt.Fprint(w, fn(str))
}

const listHeight = 14

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

const (
	Ascending  Direction = "Ascending"
	Descending           = "Descending"
)

func (i item) FilterValue() string { return "" }

type item string

type SortMessage struct {
	Choice    string
	Direction Direction
}

type Model struct {
	parentModel tea.Model
	list        list.Model
	candidates  []string
	choice      string
	direction   Direction
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case tea.KeyCtrlC.String():
			return m, tea.Quit
		case tea.KeyEnter.String():
			if m.choice != "" {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					m.direction = Direction(i)
					sm := SortMessage{
						Choice:    m.choice,
						Direction: m.direction,
					}
					m.parentModel, _ = m.parentModel.Update(sm)
					return m.parentModel, nil
				}

			} else {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					m.choice = string(i)
					m.list = m.toDirectionMode()
				}
			}

		}
	}

	m.list, _ = m.list.Update(msg)
	return m, nil
}

func (m Model) toDirectionMode() list.Model {
	const defaultWidth = 50
	l := list.New([]list.Item{item(Descending), item(Ascending)}, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Sort Direction"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	return l
}

func (m Model) View() string {
	return "\n" + m.list.View()
}

func New(candidates []string, parentModel tea.Model) Model {
	items := make([]list.Item, len(candidates))
	for i, c := range candidates {
		items[i] = item(c)
	}
	const defaultWidth = 50
	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select Sort Field"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	return Model{
		list:        l,
		direction:   "",
		choice:      "",
		candidates:  candidates,
		parentModel: parentModel,
	}

}
