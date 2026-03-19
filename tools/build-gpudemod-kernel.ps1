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
$outLib = Join-Path $OutDir 'gpudemod_kernels.lib'

Write-Host "Using nvcc: $nvcc"
Write-Host "Building $Source -> $outObj"

$nvccArgs = @('-c', $Source, '-o', $outObj, '-I', (Join-Path $CudaRoot 'include'))
if ($HostCompiler) {
  Write-Host "Using host compiler: $HostCompiler"
  $hostDir = Split-Path -Parent $HostCompiler
  $nvccArgs += @('-ccbin', $hostDir)
} else {
  $nvccArgs += @('-Xcompiler', '/EHsc')
}

& $nvcc @nvccArgs
if ($LASTEXITCODE -ne 0) {
  throw "nvcc failed with exit code $LASTEXITCODE"
}

if ($HostCompiler) {
  $ar = Get-Command ar.exe -ErrorAction SilentlyContinue
  if (-not $ar) {
    throw "ar.exe not found in PATH; required for MinGW-compatible archive"
  }
  Write-Host "Archiving $outObj -> $outLib with ar.exe"
  if (Test-Path $outLib) { Remove-Item $outLib -Force }
  & $ar 'rcs' $outLib $outObj
  if ($LASTEXITCODE -ne 0) {
    throw "ar.exe failed with exit code $LASTEXITCODE"
  }
} else {
  $libexe = Get-Command lib.exe -ErrorAction SilentlyContinue
  if (-not $libexe) {
    throw "lib.exe not found in PATH; run from vcvars64.bat environment"
  }
  Write-Host "Archiving $outObj -> $outLib with lib.exe"
  & $libexe /nologo /OUT:$outLib $outObj
  if ($LASTEXITCODE -ne 0) {
    throw "lib.exe failed with exit code $LASTEXITCODE"
  }
}

Write-Host "Built: $outObj"
Write-Host "Archived: $outLib"
