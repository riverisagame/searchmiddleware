package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// Handler 返回 Web GUI 静态文件服务（前端构建产物 embed 进单二进制）
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
