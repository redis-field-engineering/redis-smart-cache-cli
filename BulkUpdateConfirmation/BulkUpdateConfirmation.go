package BulkUpdateConfirmation

import (
	"fmt"
	"smart-cache-cli/RedisCommon"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
)

func (m Model) Init() tea.Cmd {
	return nil
}

type BulkConfirmationMessage struct {
	Message         string
	ConfirmedUpdate bool
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()
		switch s {
		case tea.KeyEsc.String(), tea.KeyCtrlC.String(), "q":
			return m, tea.Quit
		case "y", "Y":
			RedisCommon.UpdateRules(m.rdb, m.rulesToAdd, m.rulesToUpdate, m.rulesToDelete, m.applicationName)
			m.parentModel, _ = m.parentModel.Update(BulkConfirmationMessage{
				Message:         "Rule Updates Committed to Redis.",
				ConfirmedUpdate: true,
			})
			return m.parentModel, cmd
		case "n", "N":
			m.parentModel.Update(BulkConfirmationMessage{
				ConfirmedUpdate: false,
			})
			return m.parentModel, cmd
		}
	}

	return m, cmd
}

func (m Model) View() string {
	body := strings.Builder{}

	if len(m.rulesToAdd) > 0 {
		body.WriteString("====== Rules to Add ======\n")
		for _, r := range m.rulesToAdd {
			body.WriteString(r.Formatted() + "\n")
		}
	}

	if len(m.rulesToUpdate) > 0 {
		body.WriteString("\n\n====== Rules To Update ======\n")
		for _, r := range m.rulesToUpdate {
			body.WriteString(fmt.Sprintf("%s\n", r.Formatted()))
		}
	}

	if len(m.rulesToDelete) > 0 {
		body.WriteString("\n\n====== Rules to Delete ======\n")
		for _, r := range m.rulesToDelete {
			body.WriteString(fmt.Sprintf("%s\n", r.Formatted()))
		}
	}

	body.WriteString("Do you want to continue? (y/N)")
	return body.String()
}

type Model struct {
	parentModel     tea.Model
	inputMode       textinput.Model
	rulesToAdd      []RedisCommon.Rule
	rulesToUpdate   map[int]RedisCommon.Rule
	rulesToDelete   map[int]RedisCommon.Rule
	rdb             *redis.Client
	applicationName string
}

func New(parentModel tea.Model, rulesToAdd []RedisCommon.Rule, rulesToUpdate map[int]RedisCommon.Rule, rulesToDelete map[int]RedisCommon.Rule, rdb *redis.Client, applicationName string) Model {
	ti := textinput.New()
	ti.Focus()
	return Model{
		parentModel:     parentModel,
		inputMode:       ti,
		rulesToAdd:      rulesToAdd,
		rulesToDelete:   rulesToDelete,
		rulesToUpdate:   rulesToUpdate,
		rdb:             rdb,
		applicationName: applicationName,
	}
}
