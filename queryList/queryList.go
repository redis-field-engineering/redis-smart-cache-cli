package queryList

import (
	"fmt"
	"smart-cache-cli/ConfirmationDialog"
	"smart-cache-cli/RedisCommon"
	"smart-cache-cli/SortDialog"
	"smart-cache-cli/queryTtlView"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/redis/go-redis/v9"
)

const listHeight = 14

type Model struct {
	parentModel     tea.Model
	table           table.Model
	Queries         []*RedisCommon.Query
	pendingRules    map[string]RedisCommon.Rule
	Selection       int
	rdb             *redis.Client
	committed       bool
	sortColumn      string
	sortDirection   SortDialog.Direction
	applicationName string
	width           int
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

func (m Model) updateFooter() table.Model {

	successfullyCommittedText := ""
	if m.committed {
		successfullyCommittedText = "Successfuly commited caching rules!           "
	}
	footerText := fmt.Sprintf(
		"%sPg. %d/%d - Pending Updates: %d",
		successfullyCommittedText,
		m.table.CurrentPage(),
		m.table.MaxPages(),
		len(m.pendingRules),
	)

	return m.table.WithStaticFooter(footerText)
}

func (m Model) UpdateCurrentTtl(ttl string) {
	m.table.HighlightedRow().Data["Pending Rule"] = ttl
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.table, cmd = m.table.Update(msg)

	m.table = m.updateFooter()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.table, _ = m.table.Update(msg)
		return m, nil
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case tea.KeyCtrlC.String(), "q":
			return m.parentModel, tea.Quit
		case tea.KeyTab.String(), tea.KeySpace.String(), tea.KeyEnter.String():
			m.Selection = m.table.HighlightedRow().Data["RowId"].(int)
			//m.EditMode = !m.EditMode
			return queryTtlView.New(m.Queries[m.Selection], m, m.width), cmd
		case "i":
			m.table = m.table.WithHeaderVisibility(!m.table.GetHeaderVisibility())
		case "c":
			return ConfirmationDialog.New(m, m.pendingRules), cmd
		case "s":
			return SortDialog.New(RedisCommon.GetColumnNames(), m), nil
		case tea.KeyEsc.String(), "b":
			m.parentModel, _ = m.parentModel.Update(ConfirmationDialog.ConfirmationMessage{ConfirmedUpdate: true})
			return m.parentModel, nil
		}
	case queryTtlView.SetPendingTtlMsg:
		m.table.HighlightedRow().Data["Pending Rule"] = msg.Ttl
		r, ok := m.pendingRules[msg.Ttl]
		if ok {
			r.QueryIds = append(r.QueryIds, m.Queries[m.Selection].Id)
			m.pendingRules[msg.Ttl] = r
		} else {
			m.pendingRules[msg.Ttl] = RedisCommon.Rule{
				Ttl:      msg.Ttl,
				QueryIds: []string{m.Queries[m.Selection].Id},
			}
		}
	case SortDialog.SortMessage:
		columns := RedisCommon.GetColumnsOfQuery(msg.Choice, msg.Direction)
		if msg.Direction == SortDialog.Descending {
			m.table = m.table.WithColumns(columns).SortByDesc(msg.Choice)
		} else {
			m.table = m.table.WithColumns(columns).SortByAsc(msg.Choice)
		}
	case ConfirmationDialog.ConfirmationMessage:
		if msg.ConfirmedUpdate {
			err := m.CommitRuleUpdate()
			if err == nil {
				ResetModel(&m)
				m.committed = true
			}
		}

		return m, cmd
	}

	return m, cmd
}

func ResetModel(m *Model) {
	queries, err := RedisCommon.GetQueries(m.rdb, m.applicationName)

	if err != nil {
		println(err)
	}

	rows := make([]table.Row, len(queries))
	for i, q := range queries {
		rows[i] = q.GetAsRow(i)
	}

	m.table = m.table.WithRows(rows)
	m.pendingRules = make(map[string]RedisCommon.Rule)
}

func (m Model) CommitRuleUpdate() error {
	rulesToCommit := make([]RedisCommon.Rule, 0)
	for _, rule := range m.pendingRules {
		rulesToCommit = append(rulesToCommit, rule)
	}

	_, err := RedisCommon.CommitNewRules(m.rdb, rulesToCommit, m.applicationName)
	return err
}

func (m Model) View() string {
	body := strings.Builder{}
	m.table = m.updateFooter()

	body.WriteString("Press [←/→] to move pages\n")
	body.WriteString("Press 'i' to toggle the header visibility\n")
	body.WriteString("Press 's' to change sorting\n")
	body.WriteString("Press [ENTER] to create a pending rule\n")
	body.WriteString("Press 'c' to commit selected rules\n")
	body.WriteString("Press 'b' to go back\n")
	body.WriteString("Press [CTRL+C] to quit\n\n")

	body.WriteString(m.table.View())

	body.WriteString("\n\n")

	return body.String()
}

func InitialModel(pm tea.Model, rdb *redis.Client, applicationName string, width int) Model {

	queries, err := RedisCommon.GetQueries(rdb, applicationName)

	if err != nil {
		println(err)
	}

	rows := make([]table.Row, len(queries))
	for i, q := range queries {
		rows[i] = q.GetAsRow(i)
	}
	model := Model{
		table: table.New(RedisCommon.GetColumnsOfQuery("Mean Query Time", SortDialog.Descending)).
			WithRows(rows).
			HeaderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)).
			Focused(true).
			Border(customBorder).
			WithPageSize(5).
			SortByDesc("Mean Query Time").WithTargetWidth(200),
		Queries:         queries,
		parentModel:     pm,
		pendingRules:    make(map[string]RedisCommon.Rule),
		rdb:             rdb,
		applicationName: applicationName,
		width:           width,
	}
	model.table = model.updateFooter()

	return model
}
