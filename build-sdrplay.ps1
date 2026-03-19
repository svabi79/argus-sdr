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
$sdrplayInc = 'C:\PROGRA~1\SDRplay\API\inc'
$sdrplayBin = 'C:\PROGRA~1\SDRplay\API\x64'
$env:CGO_CFLAGS = "-I$sdrplayInc"
$env:CGO_LDFLAGS = "-L$sdrplayBin -lsdrplay_api"
if (Test-Path $sdrplayBin) { $env:PATH = "$sdrplayBin;" + $env:PATH }

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

$dllCandidates = @(
  (Join-Path $PSScriptRoot 'internal\demod\gpudemod\build\gpudemod_kernels.dll'),
  (Join-Path $PSScriptRoot 'gpudemod_kernels.dll')
)
$dllDst = Join-Path $PSScriptRoot 'gpudemod_kernels.dll'
$dllSrc = $dllCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if ($dllSrc) {
  if ((Resolve-Path $dllSrc).Path -ne (Resolve-Path (Split-Path $dllDst -Parent)).Path + '\gpudemod_kernels.dll') {
    try {
      Copy-Item $dllSrc $dllDst -Force
    } catch {
      Write-Host "WARNING: could not refresh runtime DLL at $dllDst ($($_.Exception.Message))" -ForegroundColor Yellow
    }
  }
  Write-Host "CUDA DLL ready at $dllDst" -ForegroundColor Green
} else {
  Write-Host 'WARNING: gpudemod_kernels.dll not found; build succeeded but runtime GPU demod will not load.' -ForegroundColor Yellow
}

Write-Host 'Done.' -ForegroundColor Green
