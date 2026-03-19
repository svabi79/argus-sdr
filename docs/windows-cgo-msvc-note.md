# Windows CGO + MSVC note

## Verified blocker

On this machine, with:

- Go on Windows
- `CC=cl.exe`
- `CXX=cl.exe`
- Visual Studio 2019 Build Tools environment loaded via `vcvars64.bat`

The following already fails before project code is involved:

```powershell
cmd /c "call \"C:\Program Files (x86)\Microsoft Visual Studio\2019\BuildTools\VC\Auxiliary\Build\vcvars64.bat\" && set CC=cl.exe && set CXX=cl.exe && set CGO_ENABLED=1 && go build -x runtime/cgo"
```

Failure shape:

- Go/cgo invokes `cl.exe`
- but still passes GCC-style flags such as:
  - `-Wall`
  - `-Werror`
  - `-fno-stack-protector`
- MSVC rejects them, e.g.:
  - `cl : command line error D8021: invalid numeric argument '/Werror'`

## Conclusion

A fully MSVC-oriented CGO build path for the CUDA+SDRplay app is currently blocked in this environment by the Go/CGO toolchain behavior itself, not by repository code.

## Practical impact

- CUDA kernel artifact preparation on Windows works
- package-level `cufft` tests work
- full app build with MinGW linker + MSVC-built CUDA artifacts fails due to ABI/runtime mismatch
- full app build with `CC=cl.exe` also fails because `runtime/cgo` already breaks

## Recommendation

Treat this as an environment/toolchain constraint. Continue to:

- keep the repository build logic clean and separated by platform
- preserve the working default non-CUDA Windows build path
- preserve Windows CUDA artifact preparation
- prefer Linux for the first end-to-end CUDA-enabled application build if full Windows CGO/MSVC compatibility remains blocked
