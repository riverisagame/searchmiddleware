// 批量：newRequest 后立即检查 err（防 req nil panic）
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	path := `D:\prj\searchmiddleware\internal\zinc\client.go`
	b, _ := os.ReadFile(path)
	lines := strings.Split(string(b), "\n")

	var out []string
	fixed := 0
	for i, ln := range lines {
		out = append(out, ln)
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "req, err := c.newRequest(") || strings.HasPrefix(trim, "req, err := c.newRequestWithContext(") {
			// 下一行是 Header.Set 直接使用 req → 插 err 检查
			if i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if strings.Contains(next, "req.") && !strings.Contains(next, "if err") {
					out = append(out, "\tif err != nil {")
					out = append(out, "\t\treturn nil, err")
					out = append(out, "\t}")
					fixed++
				}
			}
		}
	}
	if fixed > 0 {
		os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
	}
	fmt.Printf("added err guards: %d\n", fixed)
}
