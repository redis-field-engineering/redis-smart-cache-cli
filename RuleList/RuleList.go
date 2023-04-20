package RuleList

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/redis/go-redis/v9"
	"smart-cache-cli/BulkUpdateConfirmation"
	"smart-cache-cli/ConfirmationDialog"
	"smart-cache-cli/RedisCommon"
	"smart-cache-cli/RuleDialog"
	"smart-cache-cli/SortDialog"
	"smart-cache-cli/util"
	"strings"
)

type Model struct {
	parentModel               tea.Model
	table                     table.Model
	rules                     []RedisCommon.Rule
	backupRules               map[uint64]*RedisCommon.Rule
	Selection                 int
	rdb                       *redis.Client
	committed                 bool
	sortColumn                string
	sortDirection             SortDialog.Direction
	indexesWithPendingUpdates []int
	indexesWithPendingDeletes []int
	indexesWithNewRules       []int
	applicationName           string
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

func (m Model) DeleteRow(rowId int) Model {
	if contains(rowId, m.indexesWithNewRules) {
		util.Remove(m.indexesWithNewRules, rowId)
		util.Remove(m.rules, rowId)
	} else {
		m.indexesWithPendingDeletes = append(m.indexesWithPendingDeletes, rowId)
	}
	m.table = m.RefreshRows()
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	rowId := m.table.HighlightedRow().Data["RowId"].(int)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case tea.KeyCtrlC.String(), "q":
			return m.parentModel, tea.Quit
		case "b", tea.KeyEsc.String():
			m.parentModel, _ = m.parentModel.Update(ConfirmationDialog.ConfirmationMessage{ConfirmedUpdate: true})
			return m.parentModel, nil
		case "n":
			return RuleDialog.New(m, m.rdb, nil, false, m.applicationName), nil
		case "r":
			idxInDelete := indexOf(rowId, m.indexesWithPendingDeletes)
			if idxInDelete >= 0 {
				m.indexesWithPendingDeletes = util.Remove(m.indexesWithPendingDeletes, idxInDelete)
				m.table = m.RefreshRows()
			}

			idxOfEdit := indexOf(rowId, m.indexesWithPendingUpdates)
			if idxOfEdit >= 0 {
				h := m.rules[rowId].Hash()
				backUp, ok := m.backupRules[h]
				if ok {
					m.rules[rowId] = *backUp
				}
				m.indexesWithPendingUpdates = util.Remove(m.indexesWithPendingUpdates, idxOfEdit)
				m.table = m.RefreshRows()
			}
			return m, nil
		case "d":
			m = m.DeleteRow(rowId)
			return m, nil
		case "c":
			rulesToAdd := make([]RedisCommon.Rule, len(m.indexesWithNewRules))
			for i, idx := range m.indexesWithNewRules {
				rulesToAdd[i] = m.rules[idx]
			}

			rulesToUpdate := make(map[int]RedisCommon.Rule)
			for _, idx := range m.indexesWithPendingUpdates {
				rulesToUpdate[idx+len(m.indexesWithNewRules)] = m.rules[idx]
			}

			rulesToDelete := make(map[int]RedisCommon.Rule)
			for _, idx := range m.indexesWithPendingDeletes {
				rulesToDelete[idx+len(m.indexesWithNewRules)] = m.rules[idx]
			}
			confirmationDialog := BulkUpdateConfirmation.New(m, rulesToAdd, rulesToUpdate, rulesToDelete, m.rdb, m.applicationName)
			return confirmationDialog, nil

		case tea.KeyTab.String(), tea.KeySpace.String(), tea.KeyEnter.String(), "e":
			// pop open editor
			rule := m.rules[rowId]
			return RuleDialog.New(m, m.rdb, &rule, false, m.applicationName), nil
		}
	case BulkUpdateConfirmation.BulkConfirmationMessage:
		if msg.ConfirmedUpdate {
			m.table = m.table.WithStaticFooter(msg.Message)
			m.indexesWithNewRules = m.indexesWithNewRules[:0]
			m.indexesWithPendingUpdates = m.indexesWithPendingUpdates[:0]
			m.indexesWithPendingDeletes = m.indexesWithPendingDeletes[:0]
			m = m.FreshRulesFromRedis()
			m.table = m.RefreshRows()
			return m, nil
		}
	case RuleDialog.RuleMsg:
		if m.rules[rowId].Equal(msg.Rule) {
			return m, nil
		}

		if !msg.IsNew {
			h := msg.Rule.Hash()
			cpy := m.rules[rowId]
			m.backupRules[h] = &cpy
			m.rules[rowId] = msg.Rule
			if !contains(rowId, m.indexesWithPendingUpdates) {
				m.indexesWithPendingUpdates = append(m.indexesWithPendingUpdates, rowId)
			}

			idxInDelete := indexOf(rowId, m.indexesWithPendingDeletes)
			if idxInDelete >= 0 {
				m.indexesWithPendingDeletes = util.Remove(m.indexesWithPendingDeletes, idxInDelete)
			}
		} else {
			m.rules = append([]RedisCommon.Rule{msg.Rule}, m.rules...)
			for i := range m.indexesWithNewRules {
				m.indexesWithNewRules[i]++
			}
			m.indexesWithNewRules = append(m.indexesWithNewRules, 0)

			for i := range m.indexesWithPendingDeletes {
				m.indexesWithPendingDeletes[i]++
			}

			for i := range m.indexesWithPendingUpdates {
				m.indexesWithPendingUpdates[i]++
			}
		}

		m.table = m.RefreshRows()
		return m, nil
	}

	m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	body := strings.Builder{}

	body.WriteString("Press ctrl+c or q to quit\n")
	body.WriteString("press b to go back\n")
	body.WriteString("press tab or enter or space to edit a rule\n")
	body.WriteString("press n to create a rule\n")
	body.WriteString("press c to commit rule updates\n")
	body.WriteString(m.table.View())

	return body.String()
}

func indexOf(i int, s []int) int {
	for j, k := range s {
		if k == i {
			return j
		}
	}

	return -1
}

func contains(i int, s []int) bool {
	return indexOf(i, s) >= 0
}

func (m Model) RefreshRows() table.Model {
	rows := make([]table.Row, len(m.rules))
	for i, r := range m.rules {
		if contains(i, m.indexesWithPendingUpdates) {
			rows[i] = r.AsRow(i).WithStyle(lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0")))
		} else if contains(i, m.indexesWithPendingDeletes) {
			rows[i] = r.AsRow(i).WithStyle(lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("0")))
		} else if contains(i, m.indexesWithNewRules) {
			rows[i] = r.AsRow(i).WithStyle(lipgloss.NewStyle().Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")))
		} else {
			rows[i] = r.AsRow(i)
		}
	}

	return m.table.WithRows(rows)
}

func (m Model) FreshRulesFromRedis() Model {
	m.rules, _ = RedisCommon.GetRules(m.rdb, m.applicationName)
	return m
}

func New(parentModel tea.Model, rdb *redis.Client, applicationName string) Model {
	rules, _ := RedisCommon.GetRules(rdb, applicationName)
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
			SortByAsc("RowId").
			WithTargetWidth(200),
		rules:           rules,
		parentModel:     parentModel,
		rdb:             rdb,
		backupRules:     make(map[uint64]*RedisCommon.Rule),
		applicationName: applicationName,
	}

	return model
}
