# LLVM Toolchain

仓库固定 LLVM `23.1.3`。版本、下载地址与 SHA256 只在根目录 [Taskfile.yml](../../Taskfile.yml) 的 `vars:` 声明，由 [tools/toolchain](../toolchain) 落实安装：

```powershell
go -C tools/toolchain run . bootstrap
```

- Windows：下载官方 `LLVM-23.1.3-win64.msi`（639 MB，校验 SHA256）与 `llvm-mingw-20260908-ucrt-x86_64.zip`。MSI 用 Windows Installer 的**管理安装**展开到 `.tools/llvm`：`msiexec /a <msi> /qn /norestart TARGETDIR=<.tools/llvm>`，不注册产品、不写系统目录、不需要管理员权限。Go cgo 通过 `clang-cl.cmd` 以 GNU 驱动模式调用其中的官方 `clang-cl`，并复用 LLVM-MinGW UCRT 提供的 headers、运行库和链接器。
- Linux/macOS：使用系统 `clang`、`clang++`。引导器只检查版本并写入映射，不替换系统工具链。
- 引导器把 `CC`、`CXX`、`LLVM_MINGW_ROOT`、`LLVM_CLANG_CL`、`LLVM_MAJOR` 写进 `.tools/toolchain.env`，Taskfile 再映射成环境变量，因此不需要手工设置或修改 PATH。

所需工具：

- `clang`、`clang++`
- `clang-cl`（Windows）
- `clang-format`
- `clang-tidy`
- `clangd`

`task env` 会检查这些工具的可用性与版本；缺失时给出工具名、最低版本和安装入口。

## Windows 包装器约束

`clang-cl.cmd` 有两条必须保持的约定：

- 保持 ASCII 内容和 CRLF 换行：cmd.exe 用 OEM 代码页解析批处理，非 ASCII 注释会导致命令被拆错。
- 必须传入 `-Wno-unused-command-line-argument`：Go 构建 `runtime/cgo` 自带 `-Werror`，而 clang-cl 驱动模式把 `--rtlib`、`--unwindlib` 视为未使用参数，不降级该警告时 `go test -race` 直接构建失败。
- `-resource-dir` 指向 LLVM-MinGW 的 `lib/clang/%LLVM_MAJOR%`，驱动版本必须与 `LLVM_MAJOR`（来自 Taskfile 的 `LLVM_VERSION`）一致；CI 曾因驱动 20 与资源目录 23 不匹配而失败，因此不要退回到 runner 自带的驱动。

`clang-format` 和 `clang-tidy` 的长期配置位于仓库根目录；当前没有 C/C++ 源码时，CI 只校验工具存在和版本输出。
