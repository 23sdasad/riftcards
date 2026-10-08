# 015 — Windows LLVM 改用官方 MSI 管理安装

- 状态：done
- 依赖：014
- 优先级：P1
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

014 让引导器在 Windows 上使用官方 `clang+llvm-23.1.3-x86_64-pc-windows-msvc.tar.xz`（901 MB）并解压到 `.tools/llvm`，为此引入纯 Go 的 `github.com/ulikunitz/xz` 依赖。需求方指出 Windows 应使用官方安装包。

核对上游 23.1.3 资产后确认：Windows x64 的安装包是 **MSI**（`LLVM-23.1.3-win64.msi`，639 MB，sha256 `5153f848ac87553118340f16f8d3cd4351d729bcba6e4785c6a4a9af29676de9`），该版本没有 NSIS `.exe`；发布说明也建议 Windows 使用 `LLVM-*.msi` 作为 toolchain。

MSI 可以用**管理安装**（administrative install）把文件展开到指定目录：`msiexec /a <msi> /qn TARGETDIR=<dir>`。它不注册产品、不写系统目录、不需要管理员权限，正好落在 `.tools/llvm`。因此本任务把 Windows 的获取方式切换为 MSI，并删除 xz 依赖与 tar.xz 解压分支。

## 必读

[技术栈](../contracts/tech-stack.md) · [第三方库调研](../contracts/library-research.md) · [LLVM 工具链](../../tools/llvm/README.md) · [提交规范](../standards/commits.md)。

## 范围与非目标

交付：

- `Taskfile.yml` 的 Windows LLVM 声明改为 MSI 的 URL 与 SHA256（键名体现 MSI）。
- `tools/toolchain` 增加 MSI 管理安装分支：`msiexec /a <msi> /qn TARGETDIR=<dir>`，失败时输出退出码与原始输出；解压后仍按扫描定位 `clang-cl`/`clang`。
- 删除 `github.com/ulikunitz/xz` 依赖与 `tar.xz` 解压分支，引导器回到只依赖标准库。
- 同步技术栈、库调研、LLVM 说明与 task 记录。

非目标：

- 不改变 Unix 使用系统 `clang` 的策略。
- 不引入 zstd、7-Zip 或任何外部解压工具。
- 不改变 `.tools/` 布局与 `LLVM_MAJOR` 映射。

## 前置条件与待决策

- MSI 管理安装的目录布局由 MSI 的 Directory 表决定，可能嵌套一层；定位逻辑继续用“在目标目录下扫描 `clang-cl.exe`/`clang.exe`”，不假设固定层级。
- 本机到 GitHub 资产 CDN 的链路不可用，因此 MSI 的实际下载与展开由 CI 三平台中的 Windows 验证。

## 实施步骤

1. 建 task 并登记索引。
2. 改 Taskfile 声明与 `tools/toolchain` 的组件定义。
3. 实现 MSI 管理安装分支与参数构造，补单元测试。
4. 删除 xz 依赖、`tar.xz` 分支与相关文档表述。
5. 本地跑引导器单测与 `task ci`，推送后确认 Windows CI。

## 预计改动

- 修改：`Taskfile.yml`、`tools/toolchain/{config.go,fetch.go,bootstrap.go,toolchain_test.go,go.mod,go.sum}`、`tools/llvm/README.md`、`ai-docs/contracts/{tech-stack,library-research}.md`、`ai-docs/task/014-go-toolchain-bootstrap.md`、`ai-docs/task-index.md`。
- 新增：`ai-docs/task/015-windows-llvm-msi.md`。

## 清理与兼容例外

删除 xz 依赖、`tarxz` 解压分支和文档中的 `.tar.xz` 说明；不保留两套 Windows 获取方式。

## 验收标准

- [x] Windows LLVM 由官方 MSI 经管理安装展开到 `.tools/llvm`，不写系统目录、不需要管理员权限。
- [x] 引导器只依赖 Go 标准库（`go.mod` 无 `require`，`go.sum` 已删除）。
- [x] 找不到 `clang-cl`/`clang` 时给出清晰错误，MSI 失败时输出退出码；成功判据是轮询定位到 `clang-cl`。
- [x] `task ci` 与三平台 CI 通过（Windows 用 MSI 安装的 LLVM 跑 `go test -race`，run 37824684913）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `go test ./...`（tools/toolchain） | 通过 | 通过：MSI 参数构造、非 Windows 拒绝 MSI、缓存文件名解码等用例通过；模块已无第三方依赖 |
| 2026-10-08 | Windows / `msiexec /a <不存在的包> /qn /norestart TARGETDIR=…` | 报错 | 退出码为 0：msiexec 把工作交给 Windows Installer 服务，不能只信退出码。因此解压后以“能否定位到 `clang-cl`”为成功判据并轮询等待（最多 5 分钟） |
| 2026-10-08 | Windows / `task ci` | 通过 | 通过：`task ci` 全绿（golangci-lint 0 issues、C# 构建 0 警告、`go test -race` 含 proc、toolchain 单测、`dotnet test` 4 通过） |
| 2026-10-08 | GitHub Actions run [37824684913](https://github.com/23sdasad/riftcards/actions/runs/37824684913)（`eac2a7c`） | Windows 用 MSI 安装的 LLVM 跑 race，三平台通过 | 通过：ubuntu 2m38s、windows 4m56s、macos 3m1s；Windows 从 639 MB MSI 管理安装出 `clang-cl` 并跑通 `go test -race` |

## 风险与回退

风险是 MSI 管理安装在某些 Windows 版本上需要 Windows Installer 服务或产生非预期层级。回退时恢复 `.tar.xz` + 纯 Go xz 解码器（014 已验证可用），并在 task 中记录原因。

## 决策与工作记录

- 2026-10-08：需求方指出 Windows 应使用官方安装包。核对后确认 23.1.3 的 Windows 安装包是 MSI（无 NSIS exe），管理安装可满足“装到 `.tools/`、不提权、不写系统目录”，同时能删除唯一的第三方依赖。
- 2026-10-08：实测 `msiexec` 对不存在的包也返回退出码 0（安装由 Windows Installer 服务异步执行），因此成功判据改为“解压后能定位到 `clang-cl`”，并保留退出码与输出用于诊断。
- 2026-10-08：组件就绪标记改为“版本 + 锁定哈希”，切换资产类型（tar.xz → MSI）时自动失效重装。

## 完成摘要

Windows 的 LLVM 改为官方 `LLVM-23.1.3-win64.msi`（639 MB，SHA256 固定）经 Windows Installer 管理安装展开到 `.tools/llvm`：`msiexec /a <msi> /qn /norestart TARGETDIR=<.tools/llvm>`，不注册产品、不写系统目录、不需要管理员权限。由于 msiexec 的退出码不可靠，成功判据是解压后轮询定位到 `clang-cl`（最多 5 分钟）。同时删除 `github.com/ulikunitz/xz` 依赖与 `.tar.xz` 分支，引导器回到纯标准库；组件就绪标记改为“版本 + 锁定哈希”，切换资产时自动重装。三平台 CI 通过（run 37824684913：ubuntu 2m38s、windows 4m56s、macos 3m1s），Windows 用 MSI 安装的 `clang-cl` 跑通 `go test -race`；下载体积与耗时都优于原 `.tar.xz` 方案。
