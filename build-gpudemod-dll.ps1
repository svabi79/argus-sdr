$ErrorActionPreference = 'Stop'

$vcvars = 'C:\Program Files (x86)\Microsoft Visual Studio\2019\BuildTools\VC\Auxiliary\Build\vcvars64.bat'
$cudaRoot = 'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2'
$nvcc = Join-Path $cudaRoot 'bin\nvcc.exe'
$src = Join-Path $PSScriptRoot 'internal\demod\gpudemod\native\exports.cu'
$outDir = Join-Path $PSScriptRoot 'internal\demod\gpudemod\build'
$dll = Join-Path $outDir 'gpudemod_kernels.dll'
$lib = Join-Path $outDir 'gpudemod_kernels.lib'
$exp = Join-Path $outDir 'gpudemod_kernels.exp'

if (!(Test-Path $vcvars)) { throw "vcvars64.bat not found at $vcvars" }
if (!(Test-Path $nvcc)) { throw "nvcc.exe not found at $nvcc" }
if (!(Test-Path $src)) { throw "CUDA source not found at $src" }
if (!(Test-Path $outDir)) { New-Item -ItemType Directory -Path $outDir | Out-Null }

Remove-Item $dll,$lib,$exp -Force -ErrorAction SilentlyContinue

$bat = Join-Path $env:TEMP 'build-gpudemod-dll.bat'
$batContent = @"
@echo off
call "$vcvars"
if errorlevel 1 exit /b %errorlevel%
"$nvcc" -shared "$src" -o "$dll" -cudart=hybrid -Xcompiler "/MD" -arch=sm_75 ^
  -gencode arch=compute_75,code=sm_75 ^
  -gencode arch=compute_80,code=sm_80 ^
  -gencode arch=compute_86,code=sm_86 ^
  -gencode arch=compute_89,code=sm_89 ^
  -gencode arch=compute_90,code=sm_90
exit /b %errorlevel%
"@
Set-Content -Path $bat -Value $batContent -Encoding ASCII

Write-Host 'Building gpudemod CUDA DLL...' -ForegroundColor Cyan
cmd.exe /c ""$bat""
$exitCode = $LASTEXITCODE
Remove-Item $bat -Force -ErrorAction SilentlyContinue
if ($exitCode -ne 0) { throw 'gpudemod DLL build failed' }

Write-Host "Built: $dll" -ForegroundColor Green
