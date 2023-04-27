/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"smart-cache-cli/RedisCommon"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/spf13/cobra"
)

// listtablesCmd represents the listtables command
var listtablesCmd = &cobra.Command{
	Use:   "listtables",
	Short: "List the tables being profiled by Redis Smart Cache",
	Long:  `List the tables being profiled by Redis Smart Cache`,
	Run: func(cmd *cobra.Command, args []string) {
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
		})

		tables := RedisCommon.GetTables(rdb, ApplicationName)

		sbLower := strings.ToLower(sortby)
		sdLower := strings.ToLower(sortDirection)

		if sdLower != string(desc) && sdLower != asc {
			fmt.Println(fmt.Sprintf("%s is not a valid sort order. Valid orders are 'ASC' and 'DESC'.", sortDirection))
			os.Exit(1)
		}

		switch sbLower {
		case string(queryTime):
			sort.Slice(tables, func(i int, j int) bool {
				if sdLower == string(desc) {
					return tables[i].QueryTime > tables[j].QueryTime
				} else {
					return tables[i].QueryTime < tables[j].QueryTime
				}
			})
		case accessFrequency:
			sort.Slice(tables, func(i int, j int) bool {
				if sdLower == string(desc) {
					return tables[i].AccessFrequency > tables[j].AccessFrequency
				} else {
					return tables[i].AccessFrequency < tables[j].AccessFrequency
				}
			})
		}

		fmt.Println(RedisCommon.GetTablesTableHeader(20))
		for _, t := range tables {
			fmt.Println(t.GetRow(20))
		}
	},
}

func init() {
	listtablesCmd.Flags().StringVarP(&sortby, "sortby", "b", "queryTime", "The field in the"+
		" tables table to use to sort. Valid options include 'queryTime', 'accessFrequency'")
	listtablesCmd.Flags().StringVarP(&sortDirection, "sortDirection", "d", "DESC", "the direction to "+
		"sort. Valid options are 'ASC' and 'DESC'.")

	rootCmd.AddCommand(listtablesCmd)
}
