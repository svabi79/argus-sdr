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
$cudaBinX64 = 'C:\CUDA\bin\x64'
if (-not (Test-Path $cudaInc)) { $cudaInc = 'C:\PROGRA~1\NVIDIA~2\CUDA\v13.2\include' }
if (-not (Test-Path $cudaBin)) { $cudaBin = 'C:\PROGRA~1\NVIDIA~2\CUDA\v13.2\bin' }
if (-not (Test-Path $cudaBinX64)) { $cudaBinX64 = 'C:\PROGRA~1\NVIDIA~2\CUDA\v13.2\bin\x64' }
$cudaMingw = Join-Path $PSScriptRoot 'cuda-mingw'
if (Test-Path $cudaInc) { $env:CGO_CFLAGS = "$env:CGO_CFLAGS -I$cudaInc" }
if (Test-Path $cudaBinX64) { $env:PATH = "$cudaBinX64;" + $env:PATH }
if (Test-Path $cudaBin) { $env:PATH = "$cudaBin;" + $env:PATH }
if (Test-Path $cudaMingw) { $env:CGO_LDFLAGS = "$env:CGO_LDFLAGS -L$cudaMingw -lcudart64_13 -lcufft64_12 -lkernel32" }

# Fix for GCC 15 / MSYS2: ensure system headers are found by CGO
$mingwSysInclude = 'C:\msys64\mingw64\include'
if (Test-Path $mingwSysInclude) {
  $env:CGO_CFLAGS = "$env:CGO_CFLAGS -I$mingwSysInclude"
}
$mingwCrtInclude = 'C:\msys64\mingw64\x86_64-w64-mingw32\include'
if (Test-Path $mingwCrtInclude) {
  $env:CGO_CFLAGS = "$env:CGO_CFLAGS -I$mingwCrtInclude"
}

Write-Host 'Building SDRplay + cuFFT app (Windows DLL path)...' -ForegroundColor Cyan
go build -tags "sdrplay,cufft" ./cmd/sdrd
if ($LASTEXITCODE -ne 0) { throw 'build failed' }

$exePath = Join-Path $PSScriptRoot 'sdrd.exe'
$exeDir = Split-Path $exePath -Parent
$dllCandidates = @(
  (Join-Path $PSScriptRoot 'internal\demod\gpudemod\build\gpudemod_kernels.dll'),
  (Join-Path $PSScriptRoot 'gpudemod_kernels.dll')
)
$dllDst = Join-Path $exeDir 'gpudemod_kernels.dll'
$dllSrc = $dllCandidates | Where-Object { Test-Path $_ } | Sort-Object { (Get-Item $_).LastWriteTimeUtc } -Descending | Select-Object -First 1
if ($dllSrc) {
  $resolvedSrc = (Resolve-Path $dllSrc).Path
  $resolvedDst = $dllDst
  try {
    if ((Test-Path $dllDst) -and ((Resolve-Path $dllDst).Path -eq $resolvedSrc)) {
      Write-Host "CUDA DLL already current at $dllDst" -ForegroundColor Green
    } else {
      Copy-Item $dllSrc $dllDst -Force
      Write-Host "CUDA DLL copied to $dllDst" -ForegroundColor Green
    }
  } catch {
    Write-Host "WARNING: could not refresh runtime DLL at $dllDst ($($_.Exception.Message))" -ForegroundColor Yellow
  }
} else {
  Write-Host 'WARNING: gpudemod_kernels.dll not found; build succeeded but runtime GPU demod will not load.' -ForegroundColor Yellow
}

$cudartCandidates = @(
  (Join-Path $cudaBinX64 'cudart64_13.dll'),
  (Join-Path $cudaBin 'cudart64_13.dll'),
  'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin\x64\cudart64_13.dll',
  'C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2\bin\cudart64_13.dll',
  'C:\CUDA\bin\x64\cudart64_13.dll',
  'C:\CUDA\bin\cudart64_13.dll'
)
$cudartSrc = $cudartCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
if ($cudartSrc) {
  $cudartDst = Join-Path $exeDir 'cudart64_13.dll'
  try {
    Copy-Item $cudartSrc $cudartDst -Force
    Write-Host "CUDA runtime copied to $cudartDst" -ForegroundColor Green
  } catch {
    Write-Host "WARNING: could not copy CUDA runtime DLL to $cudartDst ($($_.Exception.Message))" -ForegroundColor Yellow
  }
} else {
  Write-Host 'WARNING: cudart64_13.dll not found; shared CUDA runtime may fail to load at runtime.' -ForegroundColor Yellow
}

Write-Host 'Done.' -ForegroundColor Green
