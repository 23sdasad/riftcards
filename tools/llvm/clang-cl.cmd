@echo off
setlocal

if "%LLVM_CLANG_CL%"=="" set "LLVM_CLANG_CL=C:\Program Files\LLVM\bin\clang-cl.exe"
if "%LLVM_MINGW_ROOT%"=="" (
  echo LLVM_MINGW_ROOT must point to the LLVM-MinGW installation. 1>&2
  exit /b 1
)

rem Keep this file ASCII-only: cmd.exe parses it in the OEM code page.
rem Go builds runtime/cgo with -Werror; in clang-cl driver mode --rtlib and
rem --unwindlib count as unused command line arguments, so that warning must be
rem downgraded or the race build fails.
"%LLVM_CLANG_CL%" --driver-mode=gcc --target=x86_64-w64-windows-gnu --sysroot="%LLVM_MINGW_ROOT%" -resource-dir="%LLVM_MINGW_ROOT%\lib\clang\23" --rtlib=compiler-rt --unwindlib=libunwind -Wno-unknown-argument -Wno-unused-command-line-argument -Wno-unused-macros -Wno-reserved-identifier -Wno-reserved-macro-identifier %*
exit /b %ERRORLEVEL%
