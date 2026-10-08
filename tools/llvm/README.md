# LLVM Toolchain

项目固定使用官方 LLVM `23.1.3`。Linux 和 macOS 使用 `clang`、`clang++`；Windows 使用官方 `clang-cl`，Go cgo 通过 `clang-cl.cmd` 以 GNU 驱动模式调用它，并复用 LLVM-MinGW UCRT `23.1.1-20260908` 提供的 headers、运行库和链接器。

所需工具：

- `clang`、`clang++`
- `clang-cl`
- `clang-format`
- `clang-tidy`
- `clangd`

Windows 本地验证前设置：

```powershell
$env:LLVM_MINGW_ROOT = "<llvm-mingw UCRT 解压目录>"
$env:LLVM_CLANG_CL = "<LLVM 安装目录>\bin\clang-cl.exe"
$env:CC = "<仓库>\tools\llvm\clang-cl.cmd"
$env:CXX = $env:CC
$env:CGO_ENABLED = "1"
```

Linux/macOS 本地验证前设置：

```sh
export CC=clang
export CXX=clang++
export CGO_ENABLED=1
```

`clang-format` 和 `clang-tidy` 的长期配置位于仓库根目录；当前没有 C/C++ 源码时，CI 只校验工具存在和版本输出。
