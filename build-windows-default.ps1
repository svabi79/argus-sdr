$ErrorActionPreference = 'Stop'

$gcc = 'C:\msys64\mingw64\bin'
if (-not (Test-Path (Join-Path $gcc 'gcc.exe'))) {
  throw "gcc not found at $gcc"
}

$env:PATH = "$gcc;" + $env:PATH
$env:CGO_ENABLED = '1'

Write-Host "Building default Windows app path (no CUDA artifact integration assumptions)..." -ForegroundColor Cyan
go build ./cmd/sdrd
if ($LASTEXITCODE -ne 0) { throw "default windows build failed" }
Write-Host "Done." -ForegroundColor Green
