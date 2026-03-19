$ErrorActionPreference = 'Stop'

$llvm = 'C:\Program Files\LLVM\bin'
$mingw = 'C:\msys64\mingw64'
$gccBin = Join-Path $mingw 'bin'

if (!(Test-Path (Join-Path $llvm 'clang.exe'))) { throw "clang.exe not found at $llvm" }
if (!(Test-Path (Join-Path $gccBin 'gcc.exe'))) { throw "gcc.exe not found at $gccBin" }

$cudaInc = 'C:\PROGRA~1\NVIDIA~2\CUDA\v13.2\include'
$sdrplayInc = 'C:\PROGRA~1\SDRplay\API\inc'
$sdrplayLib = 'C:\PROGRA~1\SDRplay\API\x64'

$env:PATH = "$llvm;$gccBin;" + $env:PATH
$env:CGO_ENABLED = '1'
$env:CC = 'clang --target=x86_64-w64-windows-gnu --sysroot=C:/msys64/mingw64'
$env:CXX = 'clang++ --target=x86_64-w64-windows-gnu --sysroot=C:/msys64/mingw64'
$env:CPATH = "$cudaInc;$sdrplayInc"
$env:C_INCLUDE_PATH = "$cudaInc;$sdrplayInc"
$env:CPLUS_INCLUDE_PATH = "$cudaInc;$sdrplayInc"
$env:CGO_CFLAGS = "--sysroot=C:/msys64/mingw64 -I$cudaInc -I$sdrplayInc"
$env:CGO_CPPFLAGS = "--sysroot=C:/msys64/mingw64 -I$cudaInc -I$sdrplayInc"
$env:CGO_CXXFLAGS = "--sysroot=C:/msys64/mingw64 -I$cudaInc -I$sdrplayInc"
$env:CGO_LDFLAGS = "--sysroot=C:/msys64/mingw64 -L$sdrplayLib -lsdrplay_api"

Write-Host "Testing runtime/cgo with clang GNU target..." -ForegroundColor Cyan
go build runtime/cgo
if ($LASTEXITCODE -ne 0) { throw "runtime/cgo build failed" }

Write-Host "Preparing CUDA kernel artifacts..." -ForegroundColor Cyan
powershell -ExecutionPolicy Bypass -File tools\build-gpudemod-kernel.ps1
if ($LASTEXITCODE -ne 0) { throw "kernel build failed" }

Write-Host "Building Windows CUDA + SDRplay app with clang GNU target..." -ForegroundColor Cyan
go build -tags "sdrplay,cufft" ./cmd/sdrd
if ($LASTEXITCODE -ne 0) { throw "windows cuda clang build failed" }

Write-Host "Done." -ForegroundColor Green
