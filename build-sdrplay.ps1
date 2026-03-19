$ErrorActionPreference = 'Stop'
$gcc = 'C:\msys64\mingw64\bin'
if (-not (Test-Path (Join-Path $gcc 'gcc.exe'))) {
  throw "gcc not found at $gcc"
}
if (-not (Test-Path (Join-Path $gcc 'g++.exe'))) {
  throw "g++ not found at $gcc"
}
$env:PATH = "$gcc;" + $env:PATH
$env:CGO_ENABLED = '1'
$env:CC = 'gcc'
$env:CXX = 'g++'

# SDRplay
$env:CGO_CFLAGS = '-IC:\PROGRA~1\SDRplay\API\inc'
$env:CGO_LDFLAGS = '-LC:\PROGRA~1\SDRplay\API\x64 -lsdrplay_api'

# CUDA (cuFFT)
$cudaInc = 'C:\CUDA\include'
$cudaBin = 'C:\CUDA\bin'
if (-not (Test-Path $cudaInc)) {
  $cudaInc = 'C:\PROGRA~1\NVIDIA GPU Computing Toolkit\CUDA\v13.2\include'
  $cudaBin = 'C:\PROGRA~1\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin'
}
if (Test-Path $cudaInc) {
  $env:CGO_CFLAGS = "$env:CGO_CFLAGS -I$cudaInc"
}
if (Test-Path $cudaBin) {
  $env:PATH = "$cudaBin;" + $env:PATH
}

$cudaMingw = Join-Path $PSScriptRoot 'cuda-mingw'
$gpuDemodBuild = Join-Path $PSScriptRoot 'internal\demod\gpudemod\build'
if (Test-Path $cudaMingw) {
  $env:CGO_LDFLAGS = "$env:CGO_LDFLAGS -L$cudaMingw"
}
if (Test-Path $gpuDemodBuild) {
  $env:CGO_LDFLAGS = "$env:CGO_LDFLAGS -L$gpuDemodBuild"
}
$env:CGO_LDFLAGS = "$env:CGO_LDFLAGS -lgpudemod_kernels -lcufft64_12 -lcudart64_13 -lstdc++"

Write-Host 'Building with SDRplay + cuFFT support (MinGW-host CUDA path)...' -ForegroundColor Cyan
Write-Host 'Preparing GNU-compatible CUDA kernel artifacts...' -ForegroundColor Cyan
powershell -ExecutionPolicy Bypass -File tools\build-gpudemod-kernel.ps1
if ($LASTEXITCODE -ne 0) { throw 'kernel build failed' }

go build -tags "sdrplay,cufft" ./cmd/sdrd
if ($LASTEXITCODE -ne 0) { throw 'build failed' }

Write-Host 'Done.' -ForegroundColor Green
