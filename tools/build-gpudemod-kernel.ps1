param(
  [string]$CudaRoot = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2',
  [string]$Source = 'internal/demod/gpudemod/kernels.cu',
  [string]$OutDir = 'internal/demod/gpudemod/build'
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo

$nvcc = Join-Path $CudaRoot 'bin\nvcc.exe'
if (!(Test-Path $nvcc)) {
  throw "nvcc not found at $nvcc"
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$outObj = Join-Path $OutDir 'kernels.obj'

Write-Host "Using nvcc: $nvcc"
Write-Host "Building $Source -> $outObj"

& $nvcc -c $Source -o $outObj -I (Join-Path $CudaRoot 'include') -Xcompiler "/EHsc"
if ($LASTEXITCODE -ne 0) {
  throw "nvcc failed with exit code $LASTEXITCODE"
}

Write-Host "Built: $outObj"
