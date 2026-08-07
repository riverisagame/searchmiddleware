package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"searchmiddleware/internal/config"
)

type DocumentBuilder struct {
	indexCfg *config.IndexConfig
	ds       *sql.DB
	fields   map[string]FieldInfo
}

type FieldInfo struct {
	Name         string
	Type         string
	IsArray      bool
	ElementType  string
	Format       string
	Searchable   bool
	Filterable   bool
	Sortable     bool
	Aggregatable bool
	Fields       map[string]FieldInfo
	CopyTo       []string
}

type BuildResult struct {
	Docs       []map[string]interface{}
	LastCursor string
	Count      int
}

func NewDocumentBuilder(indexCfg *config.IndexConfig, ds *sql.DB) *DocumentBuilder {
	fields := make(map[string]FieldInfo)

	addField := func(name, typ string, fc config.FieldConfig) {
		fi := FieldInfo{
			Name:         name,
			Type:         typ,
			IsArray:      fc.ElementType != "",
			ElementType:  fc.ElementType,
			Format:       fc.Format,
			Searchable:   fc.Searchable,
			Filterable:   fc.Filter,
			Sortable:     fc.Sortable,
			Aggregatable: fc.Agg,
			Fields:       make(map[string]FieldInfo),
			CopyTo:       fc.CopyTo,
		}
		for subName, subFc := range fc.Fields {
			fi.Fields[subName] = FieldInfo{
				Name:         subName,
				Type:         subFc.Type,
				Searchable:   subFc.Searchable,
				Filterable:   subFc.Filter,
				Sortable:     subFc.Sortable,
				Aggregatable: subFc.Agg,
				CopyTo:       subFc.CopyTo,
			}
		}
		fields[name] = fi
	}

	for name, fc := range indexCfg.Index.Fields {
		addField(name, fc.Type, fc)
	}

	for _, attr := range indexCfg.Source.SQLAttrUint {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "keyword", Filterable: true, Sortable: true, Aggregatable: true}
		}
	}
	for _, attr := range indexCfg.Source.SQLAttrFloat {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "float", Filterable: true, Sortable: true, Aggregatable: true}
		}
	}
	for _, attr := range indexCfg.Source.SQLAttrTimestamp {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "date", Filterable: true, Sortable: true, Format: "unix_timestamp"}
		}
	}
	for _, attr := range indexCfg.Source.SQLAttrKeyword {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "keyword", Filterable: true, Sortable: true, Aggregatable: true, Searchable: false}
		}
	}
	for _, attr := range indexCfg.Source.SQLAttrArray {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "keyword", IsArray: true, ElementType: "long", Filterable: true, Aggregatable: true}
		}
	}
	for _, attr := range indexCfg.Source.SQLFieldText {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "text", Searchable: true}
		}
	}
	for _, attr := range indexCfg.Source.SQLFieldKeyword {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "keyword", Filterable: true, Aggregatable: true, Searchable: false}
		}
	}
	for _, attr := range indexCfg.Source.SQLFieldArray {
		if _, ok := fields[attr]; !ok {
			fields[attr] = FieldInfo{Name: attr, Type: "text", IsArray: true, ElementType: "string", Searchable: true}
		}
	}

	return &DocumentBuilder{
		indexCfg: indexCfg,
		ds:       ds,
		fields:   fields,
	}
}

func (b *DocumentBuilder) BuildFull(ctx context.Context) (*BuildResult, error) {
	query := b.indexCfg.Source.SQLQuery
	rows, err := b.ds.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	return b.scanRows(rows, cols)
}

func (b *DocumentBuilder) BuildIncremental(ctx context.Context, cursor string) (*BuildResult, error) {
	baseQuery := b.indexCfg.Source.SQLQuery
	incSQL := b.indexCfg.Source.SQLIncremental
	if incSQL == "" {
		incField := b.indexCfg.Source.IncrementalField
		if incField == "" {
			incField = "update_time"
		}
		incSQL = fmt.Sprintf("WHERE %s > ?", incField)
	}

	fullQuery := b.combineQueries(baseQuery, incSQL)
	rows, err := b.ds.QueryContext(ctx, fullQuery, cursor)
	if err != nil {
		return nil, fmt.Errorf("incremental query: %w", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	return b.scanRows(rows, cols)
}

func (b *DocumentBuilder) BuildByIDs(ctx context.Context, ids []interface{}) (*BuildResult, error) {
	if len(ids) == 0 {
		return &BuildResult{}, nil
	}

	baseQuery := b.indexCfg.Source.SQLQuery
	pkCol := b.getPrimaryKeyColumn(baseQuery)
	if pkCol == "" {
		return nil, fmt.Errorf("cannot determine primary key")
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	incSQL := fmt.Sprintf("WHERE %s IN (%s)", pkCol, placeholders)

	fullQuery := b.combineQueries(baseQuery, incSQL)
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := b.ds.QueryContext(ctx, fullQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("by ids query: %w", err)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	return b.scanRows(rows, cols)
}

// SourceSQL 返回原始主 SQL
func (b *DocumentBuilder) SourceSQL() string {
	return b.indexCfg.Source.SQLQuery
}

// PrimaryKey 返回主键列名（SELECT 第一列）
func (b *DocumentBuilder) PrimaryKey() string {
	return b.getPrimaryKeyColumn(b.indexCfg.Source.SQLQuery)
}

func (b *DocumentBuilder) combineQueries(base, inc string) string {
	base = strings.TrimSpace(base)
	inc = strings.TrimSpace(inc)
	baseUpper := strings.ToUpper(base)
	incUpper := strings.ToUpper(inc)

	whereIdx := strings.Index(baseUpper, "WHERE")
	if whereIdx == -1 {
		return base + " " + inc
	}

	beforeWhere := strings.TrimRight(base[:whereIdx], " ")
	afterWhere := strings.TrimSpace(base[whereIdx+5:])
	incBody := inc
	if strings.HasPrefix(incUpper, "WHERE") {
		incBody = strings.TrimSpace(inc[5:])
	}
	return beforeWhere + " WHERE " + afterWhere + " AND " + incBody
}

func (b *DocumentBuilder) getPrimaryKeyColumn(query string) string {
	queryUpper := strings.ToUpper(query)
	selectIdx := strings.Index(queryUpper, "SELECT")
	fromIdx := strings.Index(queryUpper, "FROM")
	if selectIdx == -1 || fromIdx == -1 || fromIdx < selectIdx {
		return ""
	}
	selectClause := strings.TrimSpace(query[selectIdx+6 : fromIdx])
	cols := strings.Split(selectClause, ",")
	if len(cols) > 0 {
		return strings.TrimSpace(strings.Split(cols[0], " ")[0])
	}
	return ""
}

func (b *DocumentBuilder) scanRows(rows *sql.Rows, cols []string) (*BuildResult, error) {
	var docs []map[string]interface{}
	var lastCursor string
	count := 0

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range cols {
		valuePtrs[i] = &values[i]
	}

	attrRows := b.buildAttrRows()

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		doc := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			doc[col] = b.convertValue(val, col)
			if col == b.indexCfg.Source.IncrementalField || col == "update_time" {
				lastCursor = fmt.Sprintf("%v", val)
			}
		}

		for attrName, attrRows := range attrRows {
			if pkVal, ok := doc[b.getPrimaryKeyColumn(b.indexCfg.Source.SQLQuery)]; ok {
				if attrVals, ok := attrRows[fmt.Sprintf("%v", pkVal)]; ok {
					doc[attrName] = attrVals
				}
			}
		}

		if idVal, ok := doc[b.getPrimaryKeyColumn(b.indexCfg.Source.SQLQuery)]; ok {
			doc["_id"] = fmt.Sprintf("%v", idVal)
		}

		docs = append(docs, doc)
		count++
	}

	return &BuildResult{
		Docs:       docs,
		LastCursor: lastCursor,
		Count:      count,
	}, rows.Err()
}

func (b *DocumentBuilder) buildAttrRows() map[string]map[string]string {
	result := make(map[string]map[string]string)
	for attrName, sql := range b.indexCfg.Source.SQLJoinedField {
		rows, err := b.ds.Query(sql)
		if err != nil {
			continue
		}
		cols, _ := rows.Columns()
		if len(cols) < 2 {
			rows.Close()
			continue
		}

		_ = cols[0]
		_ = cols[1]

		attrMap := make(map[string]string)
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range cols {
			valuePtrs[i] = &values[i]
		}

		for rows.Next() {
			rows.Scan(valuePtrs...)
			pk := fmt.Sprintf("%v", values[0])
			val := fmt.Sprintf("%v", values[1])
			if existing, ok := attrMap[pk]; ok {
				attrMap[pk] = existing + " " + val
			} else {
				attrMap[pk] = val
			}
		}
		rows.Close()
		result[attrName] = attrMap
	}
	return result
}

func (b *DocumentBuilder) convertValue(val interface{}, colName string) interface{} {
	if val == nil {
		return nil
	}

	field, ok := b.fields[colName]
	if !ok {
		return val
	}

	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type == "float" || field.Type == "double" {
			return float64(v.Int())
		}
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.String:
		s := v.String()
		if field.IsArray {
			parts := strings.Split(s, " ")
			result := make([]interface{}, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					result = append(result, p)
				}
			}
			return result
		}
		return s
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			t := v.Interface().(time.Time)
			if field.Format == "unix_timestamp" {
				return t.Unix()
			}
			return t.Format(time.RFC3339)
		}
		return val
	case reflect.Slice, reflect.Array:
		if field.IsArray {
			result := make([]interface{}, v.Len())
			for i := 0; i < v.Len(); i++ {
				result[i] = b.convertValue(v.Index(i).Interface(), colName)
			}
			return result
		}
		return val
	}

	return val
}
