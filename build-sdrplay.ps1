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

# CUDA runtime / cuFFT
$cudaInc = 'C:\CUDA\include'
$cudaBin = 'C:\CUDA\bin'
if (-not (Test-Path $cudaInc)) { $cudaInc = 'C:\PROGRA~1\NVIDIA~2\CUDA\v13.2\include' }
if (-not (Test-Path $cudaBin)) { $cudaBin = 'C:\PROGRA~1\NVIDIA~2\CUDA\v13.2\bin' }
$cudaMingw = Join-Path $PSScriptRoot 'cuda-mingw'
if (Test-Path $cudaInc) { $env:CGO_CFLAGS = "$env:CGO_CFLAGS -I$cudaInc" }
if (Test-Path $cudaBin) { $env:PATH = "$cudaBin;" + $env:PATH }
if (Test-Path $cudaMingw) { $env:CGO_LDFLAGS = "$env:CGO_LDFLAGS -L$cudaMingw -lcudart64_13 -lcufft64_12 -lkernel32" }

Write-Host 'Building SDRplay + cuFFT app (Windows DLL path)...' -ForegroundColor Cyan
go build -tags "sdrplay,cufft" ./cmd/sdrd
if ($LASTEXITCODE -ne 0) { throw 'build failed' }

$dllSrc = Join-Path $PSScriptRoot 'internal\demod\gpudemod\build\gpudemod_kernels.dll'
$dllDst = Join-Path $PSScriptRoot 'gpudemod_kernels.dll'
if (Test-Path $dllSrc) {
  Copy-Item $dllSrc $dllDst -Force
  Write-Host "Copied DLL to $dllDst" -ForegroundColor Green
} else {
  Write-Host 'WARNING: gpudemod_kernels.dll not found; build succeeded but runtime GPU demod will not load.' -ForegroundColor Yellow
}

Write-Host 'Done.' -ForegroundColor Green
