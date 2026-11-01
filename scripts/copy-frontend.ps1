# 从 PHP 源项目复制预编译前端资源到部署目录（public/app）。
# 用法： powershell -ExecutionPolicy Bypass -File scripts\copy-frontend.ps1 [-Source <PHP项目根目录>]
param(
    [string]$Source = "D:\WORKSPACE\NeoSkin\blessing-skin-server-Laravel13"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$appSource = Join-Path $Source "public\app"
$target = Join-Path $root "public\app"

if (-not (Test-Path $appSource)) {
    throw "找不到前端资源目录：$appSource"
}

New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
if (Test-Path $target) {
    Remove-Item $target -Recurse -Force
}
Copy-Item $appSource $target -Recurse -Force

$favicon = Join-Path $Source "public\favicon.ico"
if (Test-Path $favicon) {
    Copy-Item $favicon (Join-Path $root "public\favicon.ico") -Force
}

$count = (Get-ChildItem $target -File | Measure-Object).Count
Write-Host "已复制 $count 个前端文件到 $target"
