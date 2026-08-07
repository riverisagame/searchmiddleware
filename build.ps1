# 一键构建：前端 build → 拷贝 embed → Go 单二进制
# 用法: powershell -ExecutionPolicy Bypass -File build.ps1

$ErrorActionPreference = 'Stop'

Write-Host "== 1/3 构建前端 (web-ui) =="
Push-Location web-ui
npm run build
Pop-Location

Write-Host "== 2/3 拷贝 dist 到 embed 目录 (internal/web/dist) =="
if (Test-Path internal/web/dist) { Remove-Item internal/web/dist -Recurse -Force }
Copy-Item -Recurse web-ui/dist internal/web/dist

Write-Host "== 3/3 构建 Go 单二进制 =="
go build -o searchmiddleware.exe ./cmd/server

Write-Host "构建完成: searchmiddleware.exe（含 Web GUI embed）"
