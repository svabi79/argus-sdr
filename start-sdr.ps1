$ErrorActionPreference = "Stop"

# Paths
$env:CUDA_PATH = "C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v13.2"
$env:PATH = "$env:CUDA_PATH\bin\x64;$env:CUDA_PATH\bin;C:\Program Files\SDRplay\API\x64;C:\msys64\mingw64\bin;$env:PATH"

# Optional: uncomment to rebuild with cuFFT
# $env:CGO_ENABLED='1'
# $env:CGO_CFLAGS='-IC:/SDRPLAY/inc -IC:/CUDA/include'
# $env:CGO_LDFLAGS='-LC:/SDRPLAY/x64 -lsdrplay_api -LC:/Users/jan/Downloads/sdr-visual-suite/cuda-mingw -lcufft64_12 -lcudart64_13'
# go build -tags "sdrplay cufft" ./cmd/sdrd

# Run
Set-Location $PSScriptRoot
Start-Process -FilePath ".\sdrd.exe" -ArgumentList "-config", "config.yaml" -WorkingDirectory $PSScriptRoot
