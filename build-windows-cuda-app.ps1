$ErrorActionPreference = 'Stop'

$msvcClDir = 'C:\Program Files (x86)\Microsoft Visual Studio\2019\BuildTools\VC\Tools\MSVC\14.29.30133\bin\Hostx64\x64'
$vcvars = 'C:\Program Files (x86)\Microsoft Visual Studio\2019\BuildTools\VC\Auxiliary\Build\vcvars64.bat'
$cudaBin = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin'
$cudaInc = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\include'
$sdrplayInc = 'C:\PROGRA~1\SDRplay\API\inc'
$sdrplayLib = 'C:\PROGRA~1\SDRplay\API\x64'
$cudaMingw = Join-Path $PSScriptRoot 'cuda-mingw'

if (!(Test-Path (Join-Path $msvcClDir 'cl.exe'))) { throw "cl.exe not found at $msvcClDir" }
if (!(Test-Path $vcvars)) { throw "vcvars64.bat not found at $vcvars" }
if (!(Test-Path (Join-Path $cudaBin 'nvcc.exe'))) { throw "nvcc.exe not found at $cudaBin" }

$env:PATH = "$msvcClDir;$cudaBin;" + $env:PATH
$env:CGO_ENABLED = '1'
$env:CC = 'cl.exe'
$env:CXX = 'cl.exe'
$env:CGO_CFLAGS = "-I$cudaInc -I$sdrplayInc"
$env:CGO_LDFLAGS = "-L$sdrplayLib -lsdrplay_api -L$cudaMingw"

Write-Host "Preparing CUDA kernel artifacts..." -ForegroundColor Cyan
powershell -ExecutionPolicy Bypass -File tools\build-gpudemod-kernel.ps1
if ($LASTEXITCODE -ne 0) { throw "kernel build failed" }

Write-Host "Building Windows CUDA app with MSVC-oriented CGO path..." -ForegroundColor Cyan
Write-Host "NOTE: This path is experimental. In this environment even 'go build runtime/cgo' emits GCC-style flags that MSVC rejects." -ForegroundColor Yellow
& cmd.exe /c "call `"$vcvars`" && go build -x -tags `"sdrplay,cufft`" ./cmd/sdrd"
if ($LASTEXITCODE -ne 0) {
  throw "windows cuda app build failed (current blocker: Go CGO emits GCC-style flags that cl.exe rejects, e.g. -Werror / -Wall / -fno-stack-protector)"
}

Write-Host "Done." -ForegroundColor Green
