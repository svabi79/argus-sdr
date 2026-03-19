$ErrorActionPreference = 'Stop'

$nvcc = (Get-Command nvcc -ErrorAction SilentlyContinue).Path
if (-not $nvcc) {
  $nvcc = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin\nvcc.exe'
}
if (-not (Test-Path $nvcc)) {
  Write-Host 'nvcc not found — skipping kernel build' -ForegroundColor Yellow
  exit 0
}

$mingwRoot = 'C:\msys64\mingw64\bin'
$mingwGpp = Join-Path $mingwRoot 'g++.exe'
$ar = Join-Path $mingwRoot 'ar.exe'
if (-not (Test-Path $mingwGpp)) {
  throw 'MinGW g++ not found'
}
if (-not (Test-Path $ar)) {
  throw 'MinGW ar not found'
}

$kernelSrc = Join-Path $PSScriptRoot '..\internal\demod\gpudemod\kernels.cu'
$buildDir = Join-Path $PSScriptRoot '..\internal\demod\gpudemod\build'
if (-not (Test-Path $buildDir)) { New-Item -ItemType Directory -Path $buildDir | Out-Null }

$objFile = Join-Path $buildDir 'kernels.o'
$libFile = Join-Path $buildDir 'libgpudemod_kernels.a'
$legacyLib = Join-Path $buildDir 'gpudemod_kernels.lib'

if (Test-Path $objFile) { Remove-Item $objFile -Force }
if (Test-Path $libFile) { Remove-Item $libFile -Force }
if (Test-Path $legacyLib) { Remove-Item $legacyLib -Force }

Write-Host 'Compiling CUDA kernels with MinGW host...' -ForegroundColor Cyan
& $nvcc -ccbin $mingwGpp -c $kernelSrc -o $objFile `
  --compiler-options=-fno-exceptions `
  -arch=sm_75 `
  -gencode arch=compute_75,code=sm_75 `
  -gencode arch=compute_80,code=sm_80 `
  -gencode arch=compute_86,code=sm_86 `
  -gencode arch=compute_87,code=sm_87 `
  -gencode arch=compute_88,code=sm_88 `
  -gencode arch=compute_89,code=sm_89 `
  -gencode arch=compute_90,code=sm_90

if ($LASTEXITCODE -ne 0) { throw 'nvcc compilation failed' }

Write-Host 'Archiving GNU-compatible CUDA kernel library...' -ForegroundColor Cyan
& $ar rcs $libFile $objFile
if ($LASTEXITCODE -ne 0) { throw 'ar archive failed' }

Write-Host "Kernel object: $objFile" -ForegroundColor Green
Write-Host "Kernel library: $libFile" -ForegroundColor Green
