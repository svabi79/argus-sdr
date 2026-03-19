$ErrorActionPreference = 'Stop'

$msvcCl = 'C:\Program Files (x86)\Microsoft Visual Studio\2019\BuildTools\VC\Tools\MSVC\14.29.30133\bin\Hostx64\x64'
if (-not (Test-Path (Join-Path $msvcCl 'cl.exe'))) {
  throw "cl.exe not found at $msvcCl"
}

$cudaBin = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin'
if (-not (Test-Path (Join-Path $cudaBin 'nvcc.exe'))) {
  throw "nvcc.exe not found at $cudaBin"
}

$env:PATH = "$msvcCl;$cudaBin;" + $env:PATH

Write-Host "Building CUDA kernel artifacts for Windows..." -ForegroundColor Cyan
powershell -ExecutionPolicy Bypass -File tools\build-gpudemod-kernel.ps1
if ($LASTEXITCODE -ne 0) { throw "kernel build failed" }

Write-Host "Done. Kernel artifacts prepared." -ForegroundColor Green
Write-Host "Note: final full-app linking may still require an MSVC-compatible CGO/link strategy, not the current MinGW flow." -ForegroundColor Yellow
