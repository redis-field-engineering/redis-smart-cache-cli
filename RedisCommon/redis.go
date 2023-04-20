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
	builder.WriteString(fmt.Sprintf("Rule TTL:%s\n", r.Ttl))

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

func ToMap(res []interface{}) map[string]string {
	m := make(map[string]string, len(res)/2)
	for i := 0; i < len(res); i += 2 {
		m[res[i].(string)] = res[i+1].(string)
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
		split := strings.SplitN(key, ".", 3)
		if len(split) < 3 {
			fmt.Printf("Skipping invalid rule %s\n", res[0].Values[key])
			continue
		}

		ruleNum, err := strconv.Atoi(split[1])
		if err != nil {
			fmt.Printf("skipping rule %s invalid rule number %s\n", key)
			continue
		}

		rule, ruleInMap := ruleMap[ruleNum]
		if !ruleInMap {
			rule = Rule{}
		}

		ruleComponent := split[2]

		switch ruleComponent {
		case "tables":
			rule.Tables = strings.Split(value.(string), ",")
		case "tablesAny":
			rule.TablesAny = strings.Split(value.(string), ",")
		case "tablesAll":
			rule.TablesAll = strings.Split(value.(string), ",")
		case "queryIds":
			rule.QueryIds = strings.Split(value.(string), ",")
		case "Regex":
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

func (r Rule) SerializeToStreamMsg(ruleNum int) []string {
	ret := make([]string, r.NumArgs()*2)
	ret[0] = fmt.Sprintf("rules.%d.ttl", ruleNum)
	ret[1] = r.Ttl
	i := 2
	if r.Regex != nil {
		ret[i] = fmt.Sprintf("rules.%d.regex", ruleNum)
		ret[i+1] = *r.Regex
		i += 2
	}
	if r.TablesAny != nil {
		ret[i] = fmt.Sprintf("rules.%d.tablesAny", ruleNum)
		ret[i+1] = strings.Join(r.TablesAny, ",")
		i += 2
	}
	if r.Tables != nil {
		ret[i] = fmt.Sprintf("rules.%d.tables", ruleNum)
		ret[i+1] = strings.Join(r.Tables, ",")
		i += 2
	}
	if r.TablesAll != nil {
		ret[i] = fmt.Sprintf("rules.%d.tablesAll", ruleNum)
		ret[i+1] = strings.Join(r.TablesAll, ",")
		i += 2
	}
	if r.QueryIds != nil {
		ret[i] = fmt.Sprintf("rules.%d.queryIds", ruleNum)
		ret[i+1] = strings.Join(r.QueryIds, ",")
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
			return fmt.Errorf("unable to update rules, rules out of sync")
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
		args = append(args, "empty")
		args = append(args, "true")
	}

	xAddArgs := redis.XAddArgs{Stream: fmt.Sprintf("%s:config", applicationName), Values: args}

	_, err = rdb.XAdd(ctx, &xAddArgs).Result()
	if err != nil {
		return err
	}
	return nil
}
