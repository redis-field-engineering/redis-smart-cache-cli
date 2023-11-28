/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"smart-cache-cli/ConfirmationDialog"
	"smart-cache-cli/RedisCommon"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"

	"github.com/spf13/cobra"
)

// makeruleCmd represents the makerule command
var makeruleCmd = &cobra.Command{
	Use:   "makerule",
	Short: "Create a caching rule",
	Long:  `Creates a caching rule`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("makerule called")
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
			Protocol: 2,
		})

		rule := RedisCommon.Rule{Ttl: ttl}
		numConditions := 0
		if tablesExact != "" {
			rule.Tables = strings.Split(tablesExact, ",")
			numConditions++
		}

		if tablesAny != "" {
			rule.TablesAny = strings.Split(tablesAny, ",")
			numConditions++
		}

		if tablesAll != "" {
			rule.TablesAll = strings.Split(tablesAll, ",")
			numConditions++
		}

		if regex != "" {
			rule.Regex = &regex
			numConditions++
		}

		if queryIds != "" {
			rule.QueryIds = strings.Split(queryIds, ",")
			numConditions++
		}

		if !confirmed {
			m := ConfirmationDialog.New(nil, map[string]RedisCommon.Rule{rule.Ttl: rule})
			p := tea.NewProgram(m)
			res, err := p.Run()
			if err != nil {
				panic(err)
			}

			confirmed = res.(ConfirmationDialog.Model).Confirmed
		}

		if confirmed {
			_, err := RedisCommon.CommitNewRules(rdb, []RedisCommon.Rule{rule}, ApplicationName)
			if err != nil {
				panic(err)
			}
			fmt.Println("Successfully created caching rule.")
		}

	},
}

var (
	tablesExact string
	tablesAny   string
	tablesAll   string
	queryIds    string
	regex       string
	ttl         string
	confirmed   bool
)

func init() {
	rootCmd.AddCommand(makeruleCmd)
	makeruleCmd.Flags().StringVarP(&tablesExact, "tablesExact", "e", "", "Comma-delimited unordered set of tables. Matches if all of the tables (and no others) appear in the query.")
	makeruleCmd.Flags().StringVarP(&tablesAny, "tablesAny", "x", "", "Comma-delimited unordered set of tables. Matches if any of these tables appear in the query.")
	makeruleCmd.Flags().StringVarP(&tablesAll, "tablesAll", "l", "", "Comma-delimited unordered set of tables. Matches if all of the tables in the set appear in the query.")
	makeruleCmd.Flags().StringVarP(&queryIds, "queryIds", "q", "", "Comma-delimited unordered set of the ids of the Queries that the rule will apply to.")
	makeruleCmd.Flags().StringVarP(&regex, "regex", "r", "", "The regex to use to match this rule. If the regex matches, the rule wil apply")
	makeruleCmd.Flags().StringVarP(&ttl, "ttl", "t", "", "The time to live as a duration (e.g. 5m, 300s, 2d) to enforce as the ttl.")
	makeruleCmd.Flags().BoolVarP(&confirmed, "confirm", "y", false, "provide this flag if you don't want the interactive dialog to confirm for you before committing.")
	err := makeruleCmd.MarkFlagRequired("ttl")
	if err != nil {
		panic(err)
	}
}
