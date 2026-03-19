$ErrorActionPreference = 'Stop'

Write-Host 'Preparing Windows CUDA DLL for gpudemod (MSVC/nvcc path)...' -ForegroundColor Cyan
powershell -ExecutionPolicy Bypass -File .\build-gpudemod-dll.ps1
if ($LASTEXITCODE -ne 0) { throw 'gpudemod DLL build failed' }
Write-Host 'Done. gpudemod_kernels.dll is ready.' -ForegroundColor Green
