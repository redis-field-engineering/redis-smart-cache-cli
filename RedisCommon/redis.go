package RedisCommon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/redis/go-redis/v9"
	"hash/fnv"
	"reflect"
	"regexp"
	"smart-cache-cli/SortDialog"
	"smart-cache-cli/util"
	"sort"
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

func GetColumnNames() []string {
	return []string{
		"Id",
		"Pending Rule",
		"Key",
		"Table",
		"Sql",
		"Access Frequency",
		"Mean Query Time",
		"Current ttl",
	}
}

func CreateColumns(sortColumn string, direction SortDialog.Direction, colNames []string, colWidth int) []table.Column {
	columns := make(map[string]table.Column)

	for _, colName := range colNames {
		columns[colName] = makeColumn(colName, colName, colWidth)
	}
	_, ok := columns[sortColumn]

	if ok {
		var symbol string
		if direction == SortDialog.Ascending {
			symbol = "↑"
		} else {
			symbol = "↓"
		}
		columns[sortColumn] = makeColumn(sortColumn, fmt.Sprintf("%s %s", sortColumn, symbol), colWidth)
	}

	ret := make([]table.Column, len(colNames))
	for i, c := range colNames {
		ret[i] = columns[c]
	}

	return ret
}

func GetColumnsOfRule(sortColumn string, direction SortDialog.Direction) []table.Column {
	colWidth := 30
	colNames := []string{
		"TTL", "Tables", "Tables All", "Tables Any", "Query Ids", "Regex",
	}

	cols := CreateColumns(sortColumn, direction, colNames, colWidth)
	var symbol string
	if sortColumn == "RowId" {
		if direction == SortDialog.Ascending {
			symbol = " ↑"
		} else {
			symbol = " ↓"
		}
	}
	cols = append([]table.Column{makeColumn("RowId", fmt.Sprintf("Rule Precedence%s", symbol), colWidth)}, cols...)
	return cols
}

func GetColumnsOfQuery(sortColumn string, direction SortDialog.Direction) []table.Column {

	colNames := []string{
		"Id", "Pending Rule", "Key", "Table", "Sql", "Access Frequency", "Mean Query Time", "Current ttl",
	}

	return CreateColumns(sortColumn, direction, colNames, 20)
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
	row := "|"
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

func (r Rule) Equal(other Rule) bool {
	return reflect.DeepEqual(r, other)
}

type Rule struct {
	Tables    []string `json:"tables"`
	TablesAny []string `json:"tablesAny"`
	TablesAll []string `json:"tablesAll"`
	Regex     *string  `json:"regex"`
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

func GetQueries(rdb *redis.Client, applicationName string) ([]*Query, error) {
	res, err := rdb.Do(ctx, "TS.MGET", "WITHLABELS", "FILTER", "name=query", "stat=(count,mean)").Result()
	if err != nil {
		return nil, err
	}
	rules, err := GetRules(rdb, applicationName)
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
		id := labels["id"]

		_, exists := queries[id]

		if !exists {
			q := new(Query)
			q.Id = id
			q.Key = fmt.Sprintf("%s:queries:%s", applicationName, id)
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
		pipeResults[id] = pipe.HGetAll(ctx, fmt.Sprintf("%s:query:%s", applicationName, id))
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

func (r Rule) AsRow(rowId int) table.Row {

	rd := table.RowData{}
	rd["TTL"] = r.Ttl
	if r.Tables != nil {
		rd["Tables"] = strings.Join(r.Tables, ",")
	} else {
		rd["Tables"] = ""
	}

	if r.TablesAny != nil {
		rd["Tables Any"] = strings.Join(r.TablesAny, ",")
	} else {
		rd["Tables Any"] = ""
	}

	if r.TablesAll != nil {
		rd["Tables All"] = strings.Join(r.TablesAll, ",")
	} else {
		rd["Tables All"] = ""
	}

	if r.QueryIds != nil {
		rd["Query Ids"] = strings.Join(r.QueryIds, ",")
	} else {
		rd["Query Ids"] = ""
	}

	if r.Regex != nil {
		rd["Regex"] = r.Regex
	} else {
		rd["Regex"] = ""
	}

	rd["RowId"] = rowId

	return table.NewRow(rd)
}

func GetRules(rdb *redis.Client, applicationName string) ([]Rule, error) {
	res, err := rdb.Do(ctx, "JSON.GET", fmt.Sprintf("%s:config", applicationName), "$.rules[*]").Result()
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

		if rule.Regex != nil {
			match, err := regexp.MatchString(*rule.Regex, query.Sql)
			if err != nil {
				match = false
			}

			if match {
				query.Rule = &rule
				return
			}

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

func (r Rule) Hash() uint64 {
	h := fnv.New64a()
	h.Write([]byte(string(r.Ttl)))
	for _, s := range r.QueryIds {
		h.Write([]byte(string(s)))
	}
	for _, s := range r.Tables {
		h.Write([]byte(string(s)))
	}
	for _, s := range r.TablesAny {
		h.Write([]byte(string(s)))
	}
	for _, s := range r.TablesAll {
		h.Write([]byte(string(s)))
	}
	if r.Regex != nil {
		h.Write([]byte(string(*r.Regex)))
	}

	return h.Sum64()
}

func CommitNewRules(rdb *redis.Client, rules []Rule, applicationName string) (string, error) {
	args := []interface{}{"JSON.ARRINSERT", fmt.Sprintf("%s:config", applicationName), "$.rules", "0"}

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

func UpdateRules(rdb *redis.Client, rulesToAdd []Rule, rulesToUpdate map[int]Rule, rulesToDelete map[int]Rule, applicationName string) {
	pipeline := rdb.Pipeline()
	for index, rule := range rulesToUpdate {
		b, err := json.Marshal(rule)
		if err != nil {
			fmt.Printf("Unalbe to update rule: %s\n", err)
			continue
		}
		pipeline.Do(ctx, "JSON.SET", fmt.Sprintf("%s:config", applicationName), fmt.Sprintf("$.rules[%d]", index), string(b))
	}

	indexesToPop := make([]int, len(rulesToDelete))
	i := 0
	for index, _ := range rulesToDelete {
		indexesToPop[i] = index
		i++
	}

	sort.Slice(indexesToPop, func(i, j int) bool {
		return indexesToPop[i] > indexesToPop[j]
	})

	for _, i := range indexesToPop {
		pipeline.Do(ctx, "JSON.ARRPOP", fmt.Sprintf("%s:config", applicationName), "$.rules", i)
	}

	for _, rule := range rulesToAdd {
		b, err := json.Marshal(rule)
		if err != nil {
			fmt.Printf("Unalbe to update rule: %s\n", err)
			continue
		}
		pipeline.Do(ctx, "JSON.ARRINSERT", fmt.Sprintf("%s:config", applicationName), "$.rules", "0", string(b))
	}

	_, err := pipeline.Exec(ctx)

	if err != nil {
		fmt.Println(fmt.Sprintf("encountered error: %s\n", err))
	}
}
