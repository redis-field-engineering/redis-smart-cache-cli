package RuleList

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/redis/go-redis/v9"
	"rsccli/RedisCommon"
	"rsccli/RuleDialog"
	"rsccli/SortDialog"
	"strings"
)

type Model struct {
	parentModel   tea.Model
	table         table.Model
	Queries       []*RedisCommon.Query
	rules         []RedisCommon.Rule
	pendingRules  map[string]RedisCommon.Rule
	Selection     int
	rdb           *redis.Client
	committed     bool
	sortColumn    string
	sortDirection SortDialog.Direction
}

var (
	customBorder = table.Border{
		Top:    "─",
		Left:   "│",
		Right:  "│",
		Bottom: "─",

		TopRight:    "╮",
		TopLeft:     "╭",
		BottomRight: "╯",
		BottomLeft:  "╰",

		TopJunction:    "╥",
		LeftJunction:   "├",
		RightJunction:  "┤",
		BottomJunction: "╨",
		InnerJunction:  "╫",

		InnerDivider: "║",
	}
)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.table, cmd = m.table.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case tea.KeyCtrlC.String(), tea.KeyEsc.String(), "q":
			return m.parentModel, tea.Quit
		case tea.KeyTab.String(), tea.KeySpace.String(), tea.KeyEnter.String(), "e":
			// pop open editor
			rowId := m.table.HighlightedRow().Data["RowId"].(int)
			rule := m.rules[rowId]
			return RuleDialog.New(m, m.rdb, &rule), nil

		}
	}

	m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	body := strings.Builder{}

	body.WriteString("Press c=tr+c/esc/q to quit")
	body.WriteString("press tab/enter/space to edit a rule")
	body.WriteString("\n")
	body.WriteString(m.table.View())

	return body.String()
}

func New(parentModel tea.Model, rdb *redis.Client) Model {
	rules, _ := RedisCommon.GetRules(rdb)
	rows := make([]table.Row, len(rules))
	for i, r := range rules {
		rows[i] = r.AsRow(i)
	}

	model := Model{
		table: table.New(RedisCommon.GetColumnsOfRule("RowId", SortDialog.Ascending)).
			WithRows(rows).
			HeaderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)).
			Focused(true).
			Border(customBorder).
			WithPageSize(10).
			SortByAsc("TTL").
			WithTargetWidth(200),
		rules:       rules,
		parentModel: parentModel,
		rdb:         rdb,
	}

	return model
}
