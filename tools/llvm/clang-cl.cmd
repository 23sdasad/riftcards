@echo off
setlocal

if "%LLVM_CLANG_CL%"=="" set "LLVM_CLANG_CL=C:\Program Files\LLVM\bin\clang-cl.exe"
if "%LLVM_MINGW_ROOT%"=="" (
  echo LLVM_MINGW_ROOT must point to the LLVM-MinGW installation. 1>&2
  exit /b 1
)

"%LLVM_CLANG_CL%" --driver-mode=gcc --target=x86_64-w64-windows-gnu --sysroot="%LLVM_MINGW_ROOT%" -resource-dir="%LLVM_MINGW_ROOT%\lib\clang\23" --rtlib=compiler-rt --unwindlib=libunwind -Wno-unknown-argument -Wno-unused-macros -Wno-reserved-identifier -Wno-reserved-macro-identifier %*
exit /b %ERRORLEVEL%
