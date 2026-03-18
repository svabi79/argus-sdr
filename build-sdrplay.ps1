$ErrorActionPreference = 'Stop'
$gcc = 'C:\msys64\mingw64\bin'
if (-not (Test-Path (Join-Path $gcc 'gcc.exe'))) {
  throw "gcc not found at $gcc"
}
$env:PATH = "$gcc;" + $env:PATH
$env:CGO_ENABLED = '1'

# SDRplay
$env:CGO_CFLAGS = '-IC:\PROGRA~1\SDRplay\API\inc'
$env:CGO_LDFLAGS = '-LC:\PROGRA~1\SDRplay\API\x64 -lsdrplay_api'

# CUDA (cuFFT)
# Prefer C:\CUDA if present (no spaces)
$cudaInc = 'C:\CUDA\include'
$cudaLib = 'C:\CUDA\lib\x64'
$cudaBin = 'C:\CUDA\bin'

if (-not (Test-Path $cudaInc)) {
  $cudaInc = 'C:\PROGRA~1\NVIDIA GPU Computing Toolkit\CUDA\v13.2\include'
  $cudaLib = 'C:\PROGRA~1\NVIDIA GPU Computing Toolkit\CUDA\v13.2\lib\x64'
  $cudaBin = 'C:\PROGRA~1\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin'
}
if (Test-Path $cudaInc) {
  $env:CGO_CFLAGS = "$env:CGO_CFLAGS -I$cudaInc"
}
if (Test-Path $cudaLib) {
  $env:CGO_LDFLAGS = "$env:CGO_LDFLAGS -L$cudaLib -lcufft -lcudart"
}
if (Test-Path $cudaBin) {
  $env:PATH = "$cudaBin;" + $env:PATH
}

Write-Host "Building with SDRplay + cuFFT support..." -ForegroundColor Cyan

go build -tags "sdrplay,cufft" ./cmd/sdrd

Write-Host "Done." -ForegroundColor Green
