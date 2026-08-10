package indexer

import (
	"testing"
)

// MySQL driver 返回 []byte：text 应转 string，DECIMAL 应转 float，INT 应转 int64
func TestConvertValueMySQLBytes(t *testing.T) {
	b := &DocumentBuilder{
		fields: map[string]FieldInfo{
			"maintenance_name": {Name: "maintenance_name", Type: "text", Searchable: true},
			"price":            {Name: "price", Type: "float"},
			"site_id":          {Name: "site_id", Type: "keyword"},
			"update_time":      {Name: "update_time", Type: "date", Format: "unix_timestamp"},
		},
	}

	cases := []struct {
		name  string
		col   string
		val   interface{}
		want  interface{}
	}{
		{"中文 text", "maintenance_name", []byte("发动机维修"), "发动机维修"},
		{"decimal float", "price", []byte("150.00"), 150.0},
		{"keyword string", "site_id", []byte("1"), "1"},
		{"unix ts", "update_time", []byte("1000"), int64(1000)},
	}
	for _, c := range cases {
		got := b.convertValue(c.val, c.col)
		if got != c.want {
			t.Errorf("%s: convertValue(%q, %s) = %v (%T), want %v (%T)", c.name, c.val, c.col, got, got, c.want, c.want)
		}
	}
}
