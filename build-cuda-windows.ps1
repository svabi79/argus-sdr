$ErrorActionPreference = 'Stop'

$mingw = 'C:\msys64\mingw64\bin'
if (-not (Test-Path (Join-Path $mingw 'g++.exe'))) {
  throw "MinGW g++ not found at $mingw"
}

$cudaBin = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin'
if (-not (Test-Path (Join-Path $cudaBin 'nvcc.exe'))) {
  throw "nvcc.exe not found at $cudaBin"
}

$env:PATH = "$mingw;$cudaBin;" + $env:PATH

Write-Host 'Preparing Windows CUDA environment for gpudemod (MinGW host compiler)...' -ForegroundColor Cyan
powershell -ExecutionPolicy Bypass -File tools\build-gpudemod-kernel.ps1
if ($LASTEXITCODE -ne 0) { throw 'kernel build failed' }

Write-Host 'Done. GNU-compatible gpudemod kernel library prepared.' -ForegroundColor Green
