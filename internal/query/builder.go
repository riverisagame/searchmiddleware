package query

import (
	"fmt"
	"strings"

	"searchmiddleware/internal/config"
)

type QueryBuilder struct {
	indexCfg *config.IndexConfig
	synonyms map[string][]string
}

func NewQueryBuilder(indexCfg *config.IndexConfig, synonyms map[string][]string) *QueryBuilder {
	return &QueryBuilder{
		indexCfg: indexCfg,
		synonyms: synonyms,
	}
}

type SearchRequest struct {
	Index     string
	Keyword   string
	SiteID    *int
	Filter    map[string]interface{}
	Page      int
	Limit     int
	Sort      string
	Highlight bool
	Aggs      map[string]interface{}
}

func (b *QueryBuilder) Build(req SearchRequest) (map[string]interface{}, error) {
	// Zinc BUG-005 规避：bool 的空 filter/should 数组导致查询静默失效（0 命中），仅在有内容时添加键
	// Zinc BUG-006 规避：bool.must 嵌套 bool.should 时 boost 应用错乱（方向颠倒），keyword 查询直接用顶层 should
	boolQuery := map[string]interface{}{}

	if req.Keyword != "" {
		should := b.buildKeywordQuery(req.Keyword)
		if should != nil {
			boolQuery["should"] = should
			boolQuery["minimum_should_match"] = 1
		}
	}

	if req.SiteID != nil || (req.Filter != nil && len(req.Filter) > 0) {
		filters := []interface{}{}
		if req.SiteID != nil {
			filters = append(filters, map[string]interface{}{"term": map[string]interface{}{"site_id": *req.SiteID}})
		}
		if req.Filter != nil {
			filterQuery := b.buildFilterQuery(req.Filter)
			if filterQuery != nil {
				filters = append(filters, filterQuery)
			}
		}
		if len(filters) > 0 {
			boolQuery["filter"] = filters
		}
	}

	if len(boolQuery) == 0 {
		boolQuery["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	}

	query := map[string]interface{}{"bool": boolQuery}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	from := (page - 1) * limit

	sort := b.buildSort(req.Sort)

	body := map[string]interface{}{
		"query": query,
		"from":  from,
		"size":  limit,
		"sort":  sort,
	}

	if req.Highlight {
		body["highlight"] = b.buildHighlight()
	}

	if len(req.Aggs) > 0 {
		body["aggs"] = req.Aggs
	}

	return body, nil
}

// buildKeywordQuery 返回 bool.should 数组（每字段独立 match + 查询级 boost）
// 规避：BUG-003 multi_match field^boost 未解析 / BUG-004 fuzziness 中文失效 / BUG-006 must 嵌套 boost 错乱
func (b *QueryBuilder) buildKeywordQuery(keyword string) []interface{} {
	fields := []string{}
	boosts := map[string]float64{}

	for name, fc := range b.indexCfg.Index.Fields {
		if fc.Searchable {
			fields = append(fields, name)
			if b.indexCfg.Index.Boost != nil {
				if boost, ok := b.indexCfg.Index.Boost[name]; ok {
					boosts[name] = boost
				}
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}

	shouldQueries := []interface{}{}
	for _, f := range fields {
		boost := 1.0
		if b, ok := boosts[f]; ok {
			boost = b
		}
		shouldQueries = append(shouldQueries, map[string]interface{}{
			"match": map[string]interface{}{
				f: map[string]interface{}{
					"query": keyword,
					"boost": boost,
				},
			},
		})
	}

	if len(b.synonyms) > 0 {
		for term, syns := range b.synonyms {
			if strings.Contains(strings.ToLower(keyword), strings.ToLower(term)) {
				for _, syn := range syns {
					synShould := []interface{}{}
					for _, f := range fields {
						synShould = append(synShould, map[string]interface{}{
							"match": map[string]interface{}{
								f: map[string]interface{}{
									"query": syn,
									"boost": 0.5,
								},
							},
						})
					}
					shouldQueries = append(shouldQueries, map[string]interface{}{
						"bool": map[string]interface{}{"should": synShould},
					})
				}
			}
		}
	}

	return shouldQueries
}

func (b *QueryBuilder) buildFilterQuery(filter map[string]interface{}) map[string]interface{} {
	if len(filter) == 0 {
		return nil
	}

	boolFilter := map[string]interface{}{
		"must": []map[string]interface{}{},
	}

	for field, value := range filter {
		fieldInfo, ok := b.indexCfg.Index.Fields[field]
		if !ok {
			continue
		}

		switch v := value.(type) {
		case []interface{}:
			if len(v) == 0 {
				continue
			}
			if fieldInfo.Type == "keyword" || fieldInfo.Type == "integer" || fieldInfo.Type == "long" {
				boolFilter["must"] = append(boolFilter["must"].([]map[string]interface{}),
					map[string]interface{}{"terms": map[string]interface{}{field: v}})
			}
		case map[string]interface{}:
			rangeQuery := map[string]interface{}{}
			for op, val := range v {
				switch op {
				case "gte", "gt", "lte", "lt":
					rangeQuery[op] = val
				}
			}
			if len(rangeQuery) > 0 {
				boolFilter["must"] = append(boolFilter["must"].([]map[string]interface{}),
					map[string]interface{}{"range": map[string]interface{}{field: rangeQuery}})
			}
		default:
			if fieldInfo.Type == "keyword" || fieldInfo.Type == "integer" || fieldInfo.Type == "long" {
				boolFilter["must"] = append(boolFilter["must"].([]map[string]interface{}),
					map[string]interface{}{"term": map[string]interface{}{field: value}})
			}
		}
	}

	if len(boolFilter["must"].([]map[string]interface{})) == 0 {
		return nil
	}

	return boolFilter
}

func (b *QueryBuilder) buildSort(sortStr string) interface{} {
	if sortStr == "" || sortStr == "score" {
		return []map[string]interface{}{{"_score": map[string]interface{}{"order": "desc"}}}
	}

	parts := strings.Split(sortStr, ":")
	field := parts[0]
	order := "desc"
	if len(parts) > 1 {
		order = parts[1]
	}

	if field == "_score" {
		return []map[string]interface{}{{"_score": map[string]interface{}{"order": order}}}
	}

	return []map[string]interface{}{{field: map[string]interface{}{"order": order}}}
}

func (b *QueryBuilder) buildHighlight() map[string]interface{} {
	fields := map[string]interface{}{}
	for name, fc := range b.indexCfg.Index.Fields {
		if fc.Searchable {
			fields[name] = map[string]interface{}{}
		}
	}
	return map[string]interface{}{
		"fields":    fields,
		"pre_tags":  []string{"<b>"},
		"post_tags": []string{"</b>"},
	}
}

func (b *QueryBuilder) ParseResponse(zincResp map[string]interface{}) (map[string]interface{}, error) {
	hits := zincResp["hits"].(map[string]interface{})
	total := int64(0)
	if totalVal, ok := hits["total"]; ok {
		if totalMap, ok := totalVal.(map[string]interface{}); ok {
			if val, ok := totalMap["value"]; ok {
				total = int64(val.(float64))
			}
		} else if val, ok := totalVal.(float64); ok {
			total = int64(val)
		}
	}

	hitsArr := hits["hits"].([]interface{})
	items := make([]map[string]interface{}, 0, len(hitsArr))
	for _, hit := range hitsArr {
		h := hit.(map[string]interface{})
		item := map[string]interface{}{
			"id":     h["_id"],
			"score":  h["_score"],
			"fields": h["_source"],
		}
		if highlight, ok := h["highlight"]; ok {
			item["highlight"] = highlight
		}
		items = append(items, item)
	}

	result := map[string]interface{}{
		"total": total,
		"items": items,
	}

	if aggs, ok := zincResp["aggregations"]; ok {
		result["aggs"] = b.formatAggregations(aggs.(map[string]interface{}))
	}

	return result, nil
}

func (b *QueryBuilder) formatAggregations(aggs map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for name, agg := range aggs {
		aggMap := agg.(map[string]interface{})
		if buckets, ok := aggMap["buckets"]; ok {
			bucketsArr := buckets.([]interface{})
			formatted := make([]map[string]interface{}, 0, len(bucketsArr))
			for _, bucket := range bucketsArr {
				b := bucket.(map[string]interface{})
				formatted = append(formatted, map[string]interface{}{
					"key":       b["key"],
					"doc_count": b["doc_count"],
				})
			}
			result[name] = map[string]interface{}{"buckets": formatted}
		}
	}
	return result
}

func (b *QueryBuilder) ValidateAggs(aggs map[string]interface{}) error {
	for name, agg := range aggs {
		aggMap := agg.(map[string]interface{})
		field := ""
		if f, ok := aggMap["field"]; ok {
			field = f.(string)
		}
		if field == "" {
			return fmt.Errorf("aggregation %s missing field", name)
		}
		if _, ok := b.indexCfg.Index.Fields[field]; !ok {
			return fmt.Errorf("aggregation field %s not found in index", field)
		}
		fieldInfo := b.indexCfg.Index.Fields[field]
		if !fieldInfo.Agg {
			return fmt.Errorf("field %s not aggregatable", field)
		}
	}
	return nil
}
