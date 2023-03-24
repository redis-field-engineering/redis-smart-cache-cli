package RedisCommon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/redis/go-redis/v9"
	"regexp"
	"rsccli/util"
	"strconv"
	"strings"
)

var ctx = context.Background()

type IndexType int
type SortField string
type Direction string

const (
	hashIdx IndexType = iota
	jsonIdx
)

const (
	queryTime       = "Query Time"
	accessFrequency = "Access Frequency"
)

const (
	ascending  = "ascending"
	descending = "descending"
)

type Query struct {
	Id          string
	Table       string
	Sql         string
	Key         string
	Count       int
	MeanTime    float64
	Selected    bool
	Rule        *Rule
	PendingRule *Rule
}

func GetPendingOrEmptyString(query *Query) string {
	if query.PendingRule == nil {
		return ""
	}
	return query.PendingRule.Ttl
}

func GetTtlOrEmptyString(query *Query) string {
	if query.Rule == nil {
		return ""
	}
	return query.Rule.Ttl
}

func makeColumn(key string, title string, columnWidth int) table.Column {
	return table.NewColumn(key, title, columnWidth).WithStyle(
		lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("#88f")).
			Align(lipgloss.Center))
}

func GetColumnsOfQuery() []table.Column {
	columns := []table.Column{
		makeColumn("Id", "Id", 20),
		makeColumn("Pending Rule", "Pending Rule", 20),
		makeColumn("Key", "Key", 20),
		makeColumn("Table", "Table", 20),
		makeColumn("Sql", "Sql", 20),
		makeColumn("Access Frequency", "Access Frequency", 20),
		makeColumn("Mean Query Time", "Mean Query Time", 20),
		makeColumn("Current ttl", "Current ttl", 20),
	}

	return columns
}

func (query *Query) GetAsRow(rowId int) table.Row {
	return table.NewRow(table.RowData{
		"Id":               query.Id,
		"Pending Rule":     GetPendingOrEmptyString(query),
		"Key":              query.Key,
		"Table":            query.Table,
		"Sql":              query.Sql,
		"Access Frequency": strconv.Itoa(query.Count),
		"Mean Query Time":  fmt.Sprintf("%.2fms", query.MeanTime),
		"Current ttl":      GetTtlOrEmptyString(query),
		"RowId":            rowId,
	})
}

func (query *Query) Formatted() string {
	return fmt.Sprintf(
		`
Query Details:
Id:			%s
Pending Rule:	%s
Key:			%s
Table:			%s
Sql:			%s
Access Frequency:	%s
Mean Query Time:	%s
Current ttl:		%s
`, query.Id, GetPendingOrEmptyString(query), query.Key, query.Table, query.Sql, strconv.Itoa(query.Count), fmt.Sprintf("%.2fms", query.MeanTime), GetTtlOrEmptyString(query))
}

func (query *Query) GetRow(colWidth int) string {
	row := "|"

	row += util.CenterString(query.Id, colWidth) + "|"
	row += util.CenterString(GetPendingOrEmptyString(query), colWidth) + "|"
	row += util.CenterString(query.Key, colWidth) + "|"
	row += util.CenterString(query.Table, colWidth) + "|"
	row += util.CenterString(query.Sql, colWidth) + "|"
	row += util.CenterString(strconv.Itoa(query.Count), colWidth) + "|"
	row += util.CenterString(fmt.Sprintf("%.2fms", query.MeanTime), colWidth) + "|"
	row += util.CenterString(GetTtlOrEmptyString(query), colWidth) + "|"
	return row
}

func GetHeader(colWidth int) string {
	row := "  |"
	row += util.CenterString("id", colWidth) + "|"
	row += util.CenterString("Pending TTL", colWidth) + "|"
	row += util.CenterString("keyName", colWidth) + "|"
	row += util.CenterString("table", colWidth) + "|"
	row += util.CenterString("sql", colWidth) + "|"
	row += util.CenterString("Access Freq.", colWidth) + "|"
	row += util.CenterString("Mean Query Time", colWidth) + "|"
	row += util.CenterString("Current TTL", colWidth) + "|"
	return row
}

func (r Rule) GetJson() string {
	b, err := json.Marshal(r)
	if err != nil {
		fmt.Println("unable to serialize rule")
		panic(r)
	}

	return string(b)
}

type Rule struct {
	Tables    []string `json:"tables"`
	TablesAny []string `json:"tablesAny"`
	TablesAll []string `json:"tablesAll"`
	Regex     []string `json:"regex"`
	QueryIds  []string `json:"queryIds"`
	Ttl       string   `json:"ttl"`
}

type SearchResult struct {
	count     int64
	documents map[string]interface{}
	indexType IndexType
}

func ToLabelsMap(res []interface{}) map[string]string {
	m := make(map[string]string, len(res)/2)
	for _, item := range res {
		fvp := item.([]interface{})
		m[fvp[0].(string)] = fvp[1].(string)
	}
	return m
}

func GetQueries(rdb *redis.Client) ([]*Query, error) {
	res, err := rdb.Do(ctx, "TS.MGET", "WITHLABELS", "FILTER", "name=query", "stat=(count,mean)").Result()
	if err != nil {
		return nil, err
	}
	rules, err := GetRules(rdb)
	if err != nil {
		return nil, err
	}

	arr, ok := res.([]interface{})
	if !ok {
		return nil, errors.New("failed to parse result from Redis")
	}

	queries := make(map[string]*Query)

	for _, item := range arr {
		labelArr := item.([]interface{})[1]
		labels := ToLabelsMap(labelArr.([]interface{}))
		id := labels["query"]

		_, exists := queries[id]

		if !exists {
			q := new(Query)
			q.Id = id
			q.Key = fmt.Sprintf("smartcache:queries:%s", id)
			queries[id] = q
		}

		if labels["stat"] == "mean" {
			queries[id].MeanTime, err = strconv.ParseFloat(item.([]interface{})[2].([]interface{})[1].(string), 64)
			if err != nil {
				return nil, err
			}
		}

		if labels["stat"] == "count" {
			queries[id].Count, err = strconv.Atoi(item.([]interface{})[2].([]interface{})[1].(string))
			if err != nil {
				return nil, err
			}
		}
	}

	pipeResults := make(map[string]*redis.MapStringStringCmd)
	pipe := rdb.Pipeline()
	for id := range queries {
		pipeResults[id] = pipe.HGetAll(ctx, fmt.Sprintf("smartcache:queries:%s", id))
	}

	_, err = pipe.Exec(ctx)

	if err != nil {
		return nil, err
	}

	for id := range pipeResults {
		result, err := pipeResults[id].Result()
		if err != nil {
			continue
		}

		table, present := result["table"]
		if present {
			queries[id].Table = table
		}

		sql, present := result["sql"]
		if present {
			queries[id].Sql = sql
		}
	}
	querySlice := make([]*Query, len(queries))
	j := 0
	for k := range queries {
		MatchRule(queries[k], rules)
		querySlice[j] = queries[k]
		j++
	}

	return querySlice, nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func GetRules(rdb *redis.Client) ([]Rule, error) {
	res, err := rdb.Do(ctx, "JSON.GET", "smartcache:config", "$.rules[*]").Result()
	if err != nil {
		return nil, err
	}

	theJson := res.(string)

	var rules []Rule
	err = json.Unmarshal([]byte(theJson), &rules)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func MatchRule(query *Query, rules []Rule) {
	tables := strings.Split(query.Table, ",")
	for _, rule := range rules {
		match := true
		for _, table := range tables {
			match = match && contains(rule.Tables, table)
		}

		if match {
			query.Rule = &rule
			return
		}

		match = true
		for _, table := range rule.TablesAll {
			match = match && contains(tables, table)
		}

		if match && rule.TablesAll != nil && len(rule.TablesAll) > 0 {
			query.Rule = &rule
			return
		}

		match = false
		for _, table := range tables {
			match = match || contains(rule.TablesAny, table)
		}

		if match {
			query.Rule = &rule
			return
		}

		match = false
		for _, regex := range rule.Regex {
			matches, err := regexp.MatchString(regex, query.Sql)
			if err != nil {
				continue
			}

			match = match || matches
		}

		if match {
			query.Rule = &rule
			return
		}

		if contains(rule.QueryIds, query.Id) {
			query.Rule = &rule
			return
		}

		if rule.TablesAny == nil && rule.Tables == nil && rule.TablesAll == nil && rule.Regex == nil && rule.QueryIds == nil {
			query.Rule = &rule
			return
		}
	}
}

func NewRule(id string, ttl string) *Rule {
	return &Rule{
		QueryIds: []string{id},
		Ttl:      ttl,
	}
}

func CommitNewRules(rdb *redis.Client, rules []Rule) (string, error) {
	args := []interface{}{"JSON.ARRINSERT", "smartcache:config", "$.rules", "0"}

	for _, rule := range rules {
		b, err := json.Marshal(rule)
		if err != nil {
			fmt.Println("Unable to serialize rule, rule commit failed, exiting. . .")
		}
		ruleStr := string(b)
		args = append(args, ruleStr)
	}

	_, err := rdb.Do(ctx, args...).Result()
	if err != nil {
		return "", err
	}

	return "OK", nil
}
