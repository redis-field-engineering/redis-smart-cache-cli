/*
Copyright © 2023 Redis steve.lorello@redis.com
*/
package cmd

import (
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"smart-cache-cli/RedisCommon"
	"sort"
	"strings"

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
	Short: "List Queries being tracked by smart cache",
	Long:  `Lists the queries being tracked by `,
	Run: func(cmd *cobra.Command, args []string) {
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
		})

		queries, err := RedisCommon.GetQueries(rdb)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		sbLower := strings.ToLower(sortby)
		sdLower := strings.ToLower(sortDirection)

		if sdLower != string(desc) && sdLower != asc {
			fmt.Println(fmt.Sprintf("%s was not a valid sort direction (valid directions are asc/desc)", sortDirection))
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

var (
	sortby        string
	sortDirection string
)

func init() {
	listqCmd.Flags().StringVarP(&sortby, "sortby", "s", "queryTime", "The field in the"+
		" queries table to use to sort. Valid options include 'queryTime', 'accessFrequency', 'tables', and 'id")
	listqCmd.Flags().StringVarP(&sortDirection, "sortDirection", "d", "DESC", "the direction to "+
		"sort, valid options are 'DESC' and 'ASC'")

	rootCmd.AddCommand(listqCmd)
}
