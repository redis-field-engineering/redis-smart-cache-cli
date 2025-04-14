package RedisCommon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"regexp"
	"smart-cache-cli/SortDialog"
	"smart-cache-cli/util"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type IndexType int
type SortField string
type Direction string
type RuleType string

const (
	All       RuleType = "All"
	Regex     RuleType = "Regex"
	Tables    RuleType = "Tables Exact"
	TablesAny RuleType = "Tables Any"
	TablesAll RuleType = "Tables All"
	QueryIds  RuleType = "Query IDs"
	Unknown   RuleType = "Unknown"
)

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

type Table struct {
	Name            string
	AccessFrequency uint64
	QueryTime       float64
	Rule            *Rule
}

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

func (t Table) GetTtl() string {
	if t.Rule != nil {
		return t.Rule.Ttl
	}
	return ""
}

func MatchTableAndRule(table Table, rules []Rule) *Rule {
	for _, rule := range rules {
		if rule.TablesAny != nil {
			if contains(rule.TablesAny, table.Name) {
				return &rule
			}
		}

		if rule.TablesAll == nil && rule.Tables == nil && rule.TablesAny == nil && rule.Regex == nil && rule.QueryIds == nil {
			return &rule
		}

		if rule.Tables != nil && contains(rule.Tables, table.Name) {
			return &rule
		}

		if rule.TablesAll != nil && contains(rule.TablesAll, table.Name) {
			return &rule
		}
	}
	return nil
}

func GetTables(rdb *redis.Client, applicationName string) []Table {
	res, err := rdb.Do(ctx, "FT.AGGREGATE", fmt.Sprintf("%s-query-idx", applicationName), "*", "APPLY", "split(@table, ',')", "AS", "name", "GROUPBY", "1", "@name", "REDUCE", "SUM", "1", "count", "as", "accessFrequency", "REDUCE", "AVG", "1", "mean", "AS", "avgQueryTime").Result()

	if err != nil {
		panic(err)
	}

	rules, err := GetRules(rdb, applicationName)

	if err != nil {
		panic(err)
	}
	outerArr := res.([]interface{})
	tables := make([]Table, outerArr[0].(int64))
	for i, item := range outerArr[1:] {
		innerArr := item.([]interface{})
		dict := ToMap(innerArr)
		name, _ := dict["name"]
		accessFrequencyStr, _ := dict["accessFrequency"]
		accessFrequency, _ := strconv.ParseUint(accessFrequencyStr, 10, 64)
		avgQueryTimeStr, _ := dict["avgQueryTime"]
		avgQueryTime, _ := strconv.ParseFloat(avgQueryTimeStr, 64)
		tables[i] = Table{
			Name:            name,
			AccessFrequency: accessFrequency,
			QueryTime:       avgQueryTime,
		}
	}

	for i, table := range tables {
		tables[i].Rule = MatchTableAndRule(table, rules)
	}

	return tables
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
		"TTL", "Rule Type", "Matches",
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
		"Id", "Pending Rule", "Key", "Table", "Sql", "Access Frequency", "Mean Query Time", "Caching Enabled", "Current ttl",
	}

	return CreateColumns(sortColumn, direction, colNames, 20)
}

func GetColumnsOfTable(sortColumn string, direction SortDialog.Direction) []table.Column {
	colNames := []string{
		"Table Name",
		"Query Time",
		"Access Frequency",
		"TTL",
	}

	return CreateColumns(sortColumn, direction, colNames, 20)
}

func (t *Table) GetAsRow(rowId int) table.Row {
	return table.NewRow(table.RowData{
		"Table Name":       t.Name,
		"Query Time":       fmt.Sprintf("%.2f", t.QueryTime),
		"Access Frequency": t.AccessFrequency,
		"TTL":              t.GetTtl(),
		"RowId":            rowId,
	})
}

func (query *Query) GetAsRow(rowId int) table.Row {
	cachingEnabled := "TRUE"
	if GetTtlOrEmptyString(query) == "" {
		cachingEnabled = "FALSE"
	}
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
		"Caching Enabled":  cachingEnabled,
	})
}

func (r Rule) Formatted() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Rule Type:%s\nRule TTL:%s\n", r.GetType(), r.Ttl))

	if r.Tables != nil {
		builder.WriteString(fmt.Sprintf("Tables: %s\n", strings.Join(r.Tables, ",")))
	}

	if r.TablesAll != nil {
		builder.WriteString(fmt.Sprintf("Tables All: %s\n", strings.Join(r.TablesAll, ",")))
	}

	if r.TablesAny != nil {
		builder.WriteString(fmt.Sprintf("Tables Any: %s\n", strings.Join(r.TablesAny, ",")))
	}

	if r.QueryIds != nil {
		builder.WriteString(fmt.Sprintf("Query IDs: %s\n", strings.Join(r.QueryIds, ",")))
	}

	if r.Regex != nil {
		builder.WriteString(fmt.Sprintf("Regex: %s\n", *r.Regex))
	}

	return builder.String()
}

func (t Table) Formatted() string {
	return fmt.Sprintf(
		`
Table:
Name: 	%s
TTL: 	%s`,
		t.Name,
		t.GetTtl())
}

func splitAcrossLines(s string, width int) string {
	var substrings []string

	for i := 0; i < len(s); i += width {
		endIndex := i + width
		if endIndex > len(s) {
			endIndex = len(s)
		}

		substrings = append(substrings, s[i:endIndex])
	}

	return strings.Join(substrings, "\n")
}

func (query *Query) Formatted(width int) string {
	builder := strings.Builder{}

	builder.WriteString("=== Query Details ===\n\n")
	builder.WriteString(fmt.Sprintf("Id:\t\t\t%s\n", query.Id))
	builder.WriteString(fmt.Sprintf("Pending rule:\t%s\n", GetPendingOrEmptyString(query)))
	builder.WriteString(fmt.Sprintf("Key:\t\t\t%s\n", query.Key))
	builder.WriteString(fmt.Sprintf("Table:\t\t\t%s\n", query.Table))
	if len(query.Sql)+4 > width {
		builder.WriteString(fmt.Sprintf("SQL:\n\n%s\n\n", splitAcrossLines(query.Sql, width)))
	} else {
		builder.WriteString(fmt.Sprintf("SQL:%s\n", query.Sql))
	}
	builder.WriteString(fmt.Sprintf("Access frequency %s\n", strconv.Itoa(query.Count)))
	builder.WriteString(fmt.Sprintf("Mean query time: %.2fms\n", query.MeanTime))
	builder.WriteString(fmt.Sprintf("Current TTL: %s\n", GetTtlOrEmptyString(query)))

	return builder.String()
}

func (table *Table) GetRow(colWidth int) string {
	row := "|"
	row += util.CenterString(table.Name, colWidth) + "|"
	row += util.CenterString(strconv.FormatUint(table.AccessFrequency, 10), colWidth) + "|"
	row += util.CenterString(fmt.Sprintf("%.2f", table.QueryTime), colWidth) + "|"
	row += util.CenterString(table.GetTtl(), colWidth) + "|"
	return row
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

func GetTablesTableHeader(colWidth int) string {
	row := "|"
	row += util.CenterString("Name", colWidth) + "|"
	row += util.CenterString("Access Frequency", colWidth) + "|"
	row += util.CenterString("Query Time", colWidth) + "|"
	row += util.CenterString("TTL", colWidth) + "|"
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
		fmt.Println("Error: Unable to serialize rule.")
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

func (r Rule) Type() {

}

type SearchResult struct {
	count     int64
	documents map[string]interface{}
	indexType IndexType
}

func ToMap(res []interface{}) map[string]string {
	m := make(map[string]string, len(res)/2)
	for i := 0; i < len(res)-1; i += 2 {
		key, ok := res[i].(string)
		if !ok {
			continue // skip if the key is not a string
		}

		val, _ := res[i+1].(string) // default ""
		m[key] = val
	}
	return m
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
		return nil, errors.New("Error: Failed to parse result from Redis")
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
			q.Key = fmt.Sprintf("%s:query:%s", applicationName, id)
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

func (r Rule) GetType() RuleType {
	if r.Tables != nil {
		return Tables
	}

	if r.TablesAny != nil {
		return TablesAny
	}

	if r.TablesAll != nil {
		return TablesAll
	}

	if r.QueryIds != nil {
		return QueryIds
	}

	if r.Regex != nil {
		return Regex
	}

	return All
}

func (r Rule) AsRow(rowId int) table.Row {

	rd := table.RowData{}
	rd["TTL"] = r.Ttl
	rd["Matches"] = "any"
	rd["Rule Type"] = r.GetType()
	if r.Tables != nil {
		rd["Matches"] = strings.Join(r.Tables, ",")
	}

	if r.TablesAny != nil {
		rd["Matches"] = strings.Join(r.TablesAny, ",")
	}

	if r.TablesAll != nil {
		rd["Matches"] = strings.Join(r.TablesAll, ",")
	}

	if r.QueryIds != nil {
		rd["Matches"] = strings.Join(r.QueryIds, ",")
	}

	if r.Regex != nil {
		rd["Matches"] = *r.Regex
	}

	rd["RowId"] = rowId

	return table.NewRow(rd)
}

func GetRules(rdb *redis.Client, applicationName string) ([]Rule, error) {

	res, err := rdb.XRevRangeN(ctx, fmt.Sprintf("%s:config", applicationName), "+", "-", 1).Result()

	if err != nil {
		return nil, err
	}

	if len(res) < 1 {
		return make([]Rule, 0), nil
	}

	ruleMap := make(map[int]Rule)

	for key, _ := range res[0].Values {
		value := res[0].Values[key]
		split := strings.Split(key, ".")
		if len(split) < 3 {
			fmt.Printf("Skipping invalid rule '%s'\n", res[0].Values[key])
			continue
		}

		ruleNum, err := strconv.Atoi(split[1])
		if err != nil {
			fmt.Printf("Skipping rule '%s'. Invalid rule number %s\n", key)
			continue
		}

		rule, ruleInMap := ruleMap[ruleNum]
		if !ruleInMap {
			rule = Rule{}
		}

		ruleComponent := split[2]

		switch ruleComponent {
		case "tables":
			if rule.Tables == nil {
				rule.Tables = make([]string, 0)
			}
			rule.Tables = append(rule.Tables, value.(string))
		case "tables-any":
			if rule.TablesAny == nil {
				rule.TablesAny = make([]string, 0)
			}
			rule.TablesAny = append(rule.TablesAny, value.(string))
		case "tables-all":
			if rule.TablesAll == nil {
				rule.TablesAll = make([]string, 0)
			}
			rule.TablesAll = append(rule.TablesAll, value.(string))
		case "query-ids":
			if rule.QueryIds == nil {
				rule.QueryIds = make([]string, 0)
			}
			rule.QueryIds = append(rule.QueryIds, value.(string))
		case "regex":
			r := value.(string)
			rule.Regex = &r
		case "ttl":
			rule.Ttl = value.(string)
		}

		ruleMap[ruleNum] = rule

	}

	rules := make([]Rule, len(ruleMap))

	for i, rule := range ruleMap {
		rules[i-1] = rule
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

func (r Rule) NumArgs() int {
	num := 1 // for ttl
	if r.Regex != nil {
		num++
	}
	if r.TablesAny != nil {
		num++
	}
	if r.Tables != nil {
		num++
	}
	if r.TablesAll != nil {
		num++
	}
	if r.QueryIds != nil {
		num++
	}
	return num
}

func serializeToJacksonArr(arr []string, component string, ruleNum int) []string {
	var res []string
	for i, item := range arr {
		res = append(res, fmt.Sprintf("rules.%d.%s.%d", ruleNum, component, i+1))
		res = append(res, item)
	}

	return res
}

func (r Rule) SerializeToStreamMsg(ruleNum int) []string {
	var ret []string
	ret = append(ret, fmt.Sprintf("rules.%d.ttl", ruleNum))
	ret = append(ret, r.Ttl)
	if r.Regex != nil {
		ret = append(ret, fmt.Sprintf("rules.%d.regex", ruleNum))
		ret = append(ret, *r.Regex)
	}
	if r.TablesAny != nil {
		ret = append(ret, serializeToJacksonArr(r.TablesAny, "tables-any", ruleNum)...)
	}
	if r.Tables != nil {
		ret = append(ret, serializeToJacksonArr(r.Tables, "tables", ruleNum)...)
	}
	if r.TablesAll != nil {
		ret = append(ret, serializeToJacksonArr(r.TablesAll, "tables-all", ruleNum)...)
	}
	if r.QueryIds != nil {
		ret = append(ret, serializeToJacksonArr(r.QueryIds, "query-ids", ruleNum)...)
	}

	return ret
}

func CommitNewRules(rdb *redis.Client, rules []Rule, applicationName string) (string, error) {
	currentRules, err := GetRules(rdb, applicationName)
	if err != nil {
		panic(err)
	}

	args := make([]string, 0)

	for i, rule := range rules {
		args = append(args, rule.SerializeToStreamMsg(i+1)...)
	}

	for i, rule := range currentRules {
		args = append(args, rule.SerializeToStreamMsg(i+1+len(rules))...)
	}

	xAddArgs := redis.XAddArgs{Stream: fmt.Sprintf("%s:config", applicationName), Values: args}

	id, err := rdb.XAdd(ctx, &xAddArgs).Result()
	if err != nil {
		return "", err
	}

	return id, nil
}

func UpdateRules(rdb *redis.Client, rulesToAdd []Rule, rulesToUpdate map[int]Rule, rulesToDelete map[int]Rule, applicationName string) error {
	currentRules, err := GetRules(rdb, applicationName)

	if err != nil {
		panic(err)
	}

	rulesToCommit := make([]Rule, len(currentRules))
	copy(rulesToCommit, currentRules)

	for index, rule := range rulesToUpdate {
		if index >= len(currentRules) {
			return fmt.Errorf("Unable to update rules: rules out of sync.")
		}

		rulesToCommit[index] = rule
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
		rulesToCommit = append(rulesToCommit[:i], rulesToCommit[i+1:]...)
	}

	for _, rule := range rulesToAdd {
		rulesToCommit = append([]Rule{rule}, rulesToCommit...)
	}

	args := make([]string, 0)

	for i, rule := range rulesToCommit {
		args = append(args, rule.SerializeToStreamMsg(i+1)...)
	}

	if len(args) == 0 {
		args = append(args, "rule.1.ttl")
		args = append(args, "0s")
	}

	xAddArgs := redis.XAddArgs{Stream: fmt.Sprintf("%s:config", applicationName), Values: args}

	_, err = rdb.XAdd(ctx, &xAddArgs).Result()
	if err != nil {
		return err
	}
	return nil
}

func Ping(rdb *redis.Client) error {
	_, err := rdb.Ping(ctx).Result()
	return err
}

func CheckSmartCacheIndex(rdb *redis.Client, applicationName string) error {
	res, err := rdb.Do(ctx, "FT._LIST").Result()
	if err != nil {
		return err
	}

	arr := res.([]interface{})

	strs := make([]string, len(arr))

	for index, i := range arr {
		strs[index] = i.(string)
	}

	if !contains(strs, fmt.Sprintf("%s-query-idx", applicationName)) {
		return errors.New(fmt.Sprintf("Redis Smart Cache does not appear to be configured for application '%s'. "+
			"Please ensure that Redis Smart Cache is running, configured with application '%s', and pointed at the correct Redis instance.", applicationName, applicationName))
	}

	return nil
}
