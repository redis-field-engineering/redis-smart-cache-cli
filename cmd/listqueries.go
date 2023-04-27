/*
Copyright © 2023 Redis steve.lorello@redis.com
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

type sortAttribute string
type sortDir string

const (
	queryTime       sortAttribute = "querytime"
	accessFrequency               = "accessfrequency"
	tables                        = "tables"
	id                            = "id"
)

const (
	desc sortDir = "desc"
	asc          = "asc"
)

// listqCmd represents the listq command
var listqCmd = &cobra.Command{
	Use:   "listqueries",
	Short: "List the queries seen by Redis Smart Cache",
	Long:  `List queries seen by `,
	Run: func(cmd *cobra.Command, args []string) {
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
		})

		queries, err := RedisCommon.GetQueries(rdb, ApplicationName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		sbLower := strings.ToLower(sortby)
		sdLower := strings.ToLower(sortDirection)

		if sdLower != string(desc) && sdLower != asc {
			fmt.Println(fmt.Sprintf("%s is not a valid sort order. Valid orders are 'ASC' and 'DESC'.", sortDirection))
			os.Exit(1)
		}

		switch sbLower {
		case string(queryTime):
			sort.Slice(queries, func(i int, j int) bool {
				if sdLower == string(desc) {
					return queries[i].MeanTime > queries[j].MeanTime
				} else {
					return queries[i].MeanTime < queries[j].MeanTime
				}

			})
		case string(accessFrequency):
			sort.Slice(queries, func(i int, j int) bool {
				if sdLower == string(desc) {
					return queries[i].Count > queries[j].Count
				} else {
					return queries[i].Count < queries[j].Count
				}
			})

		}

		fmt.Println(RedisCommon.GetHeader(20))
		for _, q := range queries {
			fmt.Println(q.GetRow(20))
		}
	},
}

func init() {
	listqCmd.Flags().StringVarP(&sortby, "sortby", "b", "queryTime", "The field in the"+
		" queries table to use to sort. Valid options include 'queryTime', 'accessFrequency', 'tables', and 'id")
	listqCmd.Flags().StringVarP(&sortDirection, "sortDirection", "d", "DESC", "the direction to "+
		"sort. Valid options are 'ASC' and 'DESC'.")

	rootCmd.AddCommand(listqCmd)
}
