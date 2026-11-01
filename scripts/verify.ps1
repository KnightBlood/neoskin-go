# 一键复验：构建、静态检查、全部测试（含 HTTP E2E）。
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $root

Write-Host "== go build =="
go build ./...
Write-Host "== go vet =="
go vet ./...
Write-Host "== go test =="
go test ./... -count=1
Write-Host "全部通过。"
