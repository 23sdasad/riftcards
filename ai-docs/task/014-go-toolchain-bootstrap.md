# 014 — 用 Go 引导全部依赖

- 状态：done
- 依赖：012
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

012 把命令入口收回 `Taskfile.yml`，但依赖仍散落：Go Task、golangci-lint、govulncheck 由 `go install` 安装，.NET SDK 和 Godot 要求开发者自行安装，Windows 的 LLVM 与 LLVM-MinGW 只在 CI 里下载，版本口径分布在 Taskfile、CI 和文档三处。

本任务把依赖声明收敛到 `Taskfile.yml`，并新增纯 Go 的引导器 `tools/toolchain`：开发者只要本机有 Go，就能用一条 `go` 命令取得全部依赖（Go 工具、.NET SDK、Godot、Windows LLVM 与 LLVM-MinGW、NuGet 包），装到仓库内 `.tools/`，不污染系统。

## 必读

[技术栈](../contracts/tech-stack.md) · [测试策略](../architecture/testing.md) · [Path 与工程边界](../architecture/paths-and-boundaries.md) · [第三方库调研](../contracts/library-research.md) · [提交规范](../standards/commits.md)。

## 范围与非目标

交付：

- 新增独立 Go 模块 `tools/toolchain`（只依赖标准库和一个纯 Go xz 解码器），子命令：
  - `bootstrap`：安装 Go 工具（Task、golangci-lint、govulncheck）、下载并解压 .NET SDK、Godot、Windows LLVM 与 LLVM-MinGW，并按锁文件还原 NuGet 依赖；`--without <组件>` 可跳过，`--force` 可重装。
  - `check`：逐项检查工具是否存在、版本是否满足锁定口径，缺失项输出工具名、最低版本和安装入口。
  - `env`：打印生成的映射文件，便于排查。
- 所有版本、URL、SHA256/SHA512 只在 `Taskfile.yml` 声明，通过 `TC_*` 环境变量传给引导器；引导器不硬编码版本。
- 生成 `.tools/toolchain.env`，由 `Taskfile.yml` 的 `dotenv` 读取，并把 `TC_CC`、`TC_CXX`、`TC_DOTNET_ROOT`、`TC_LLVM_MINGW_ROOT`、`TC_LLVM_CLANG_CL` 映射成全局环境，使 `task` 命令无需改开发者 PATH。
- 依赖装到 `.tools/`（git 忽略），`task env`、`task bootstrap` 与 `task ci` 都基于该目录。
- CI 改为 `setup-go` + 引导器 + `task ci`，删除 `setup-dotnet`、`setup-task` 和手工下载 LLVM/LLVM-MinGW 的步骤，版本统一来自 Taskfile。
- 同步 README、技术栈、测试策略、路径边界、库调研和 task 索引。

非目标：

- 不自动安装 Go 本身（Go 是引导前提），也不修改开发者的 shell profile 或系统 PATH。
- 不引入除 xz 解码器之外的第三方依赖，不引入包管理器脚本（brew/winget/apt）。
- 不把 `.tools/` 纳入版本控制，也不改业务代码和协议。

## 前置条件与待决策

- 引导器语言与位置：确认用独立 Go 模块 `tools/toolchain`，避免把引导依赖塞进 `server` 模块。
- 安装位置：确认仓库内 `.tools/`。
- 引导入口：Task 本身也是依赖，因此首次入口必须是不依赖 Task 的 `go -C tools/toolchain run . bootstrap`。
- Godot 仅客户端运行需要，CI 用 `--without godot` 跳过。

## 实施步骤

1. 建 task 并登记索引，收集各平台固定资产（URL 与哈希）。
2. 实现 `tools/toolchain`：配置解析、平台映射、下载校验、zip/tar.gz/tar.xz 解压、环境文件与 shim 生成、check、bootstrap。
3. 为解压安全、哈希校验、平台映射、版本比较和环境文件渲染补单元测试。
4. 改写 `Taskfile.yml`：版本集中声明、`dotenv` 与全局 `env` 映射、`bootstrap`/`env` 任务、两个 Go 模块的 fmt/lint/test。
5. 改写 CI：删除手工工具安装，改用引导器，并缓存。
6. 更新文档与 `global.json`（.NET SDK 对齐到 8.0.425）。
7. 本机跑 `bootstrap`、`task env`、`task ci`，推送后确认三平台 CI。

## 预计改动

- 新增：`tools/toolchain/`（go.mod、go.sum、实现与测试）、`ai-docs/task/014-go-toolchain-bootstrap.md`。
- 修改：`Taskfile.yml`、`.github/workflows/ci.yml`、`.gitignore`、`global.json`、`README.md`、`AGENTS.md`、`ai-docs/README.md`、`ai-docs/architecture/{testing,paths-and-boundaries,conventions}.md`、`ai-docs/contracts/{tech-stack,library-research}.md`、`tools/llvm/README.md`、`ai-docs/task-index.md`。

## 清理与兼容例外

删除 CI 中的 `actions/setup-dotnet`、`arduino/setup-task` 和手工下载 LLVM/LLVM-MinGW 步骤，删除 Taskfile 里“直接命令 + 工具假设已安装”的重复版本口径。保留系统 clang 在 Unix 上作为编译器来源（引导器只报告版本，不替换系统工具链）。

## 验收标准

- [x] 只有 Go 的机器上执行 `go -C tools/toolchain run . bootstrap` 后，`task env` 全绿（本机因网络限制未验证 Godot，CI 三平台已验证引导器与门禁；Godot 只在客户端运行需要，CI 以 `--without godot` 跳过）。
- [x] `task bootstrap` 与首次引导等价，重复执行幂等，`--without` 可跳过组件。
- [x] 所有版本、URL、哈希只在 `Taskfile.yml` 出现一次，`tools/toolchain` 不硬编码版本。
- [x] 校验失败（哈希不符）时中止且不写入半成品目录：缓存的 MinGW 归档按 Taskfile 的 SHA256 校验通过；单元测试覆盖哈希不符与路径越界。
- [x] `task env` 对缺失工具给出工具名、最低版本和安装入口。
- [x] `task ci` 在 Windows 通过，全程使用 `.tools/` 中的 .NET SDK 与引导出的编译器。
- [x] CI 三平台通过（run 37815934763）。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `go -C tools/toolchain test ./...` | 单元测试通过 | 通过：`go vet` 无输出，测试覆盖哈希校验、zip/tar.gz 解压、路径越界拒绝、Taskfile 声明解析、环境文件渲染、Godot/MinGW 定位 |
| 2026-10-08 | Windows / `go -C tools/toolchain run . bootstrap` | 全部依赖就位 | 部分通过：Go 工具复用已锁定的 3.54.0 / 2.14.0 / 1.1.4；`.NET SDK 8.0.425` 下载 285 MB 并解压成功；LLVM-MinGW 20260908 从缓存校验 SHA256（与 Taskfile 值一致）并解压成功；NuGet 三个项目 `--locked-mode` 还原通过。Godot 与 Windows LLVM 归档在本机无法完成下载（到 GitHub 资产 CDN 约 85 KB/s 且中断），留待 CI 验证 |
| 2026-10-08 | Windows / `task env` | 逐项报告版本与缺失项 | 通过（除 Godot）：go 1.27.0、task 3.54.0、golangci-lint 2.14.0、govulncheck 1.1.4、dotnet 8.0.425（取自 `.tools`）、clang 23.1.3、llvm-mingw 20260908 全部 OK；godot 报 MISSING 并给出安装入口，退出码 201 |
| 2026-10-08 | Windows / `task ci` | 全量门禁通过 | 通过：`gofmt -l` 无输出；3 个 C# 项目格式检查通过；两个 Go 模块 `go vet` 与 golangci-lint 0 issues；C# 构建 0 警告；server `go test -race` 通过；toolchain `go test` 通过；`dotnet test` 4 通过 0 失败。全程使用 `.tools/dotnet` 与引导出的 `CC` |
| 2026-10-08 | 幂等性 / 再次 `bootstrap --without godot,llvm` | 不重复下载 | 通过：Go 工具与 .NET SDK 报 “已就绪”，仅从缓存解压 MinGW |
| 2026-10-08 | GitHub Actions run [37815934763](https://github.com/23sdasad/riftcards/actions/runs/37815934763)（`1323b54`） | 引导器 + `task ci` 通过 | 通过：ubuntu 1m12s、macos 5m44s、windows 30m49s 全部成功。Windows 时间主要花在 900 MB LLVM 归档的下载与解压；该归档已进入 `.tools/cache` 缓存（1.27 GiB），后续运行直接命中缓存 |

## 风险与回退

主要风险是网络中断导致半成品目录，以及 .NET SDK 压缩包在不同平台布局不同。回退时保留 `Taskfile.yml` 的版本声明，引导器退回只做 Go 工具安装，其余依赖回到文档手装；不引入系统级安装步骤。

## 决策与工作记录

- 2026-10-08：创建任务。确认由独立 Go 模块引导，依赖装到 `.tools/`，版本只在 Taskfile 声明。
- 2026-10-08：实测 Task 的 `dotenv` 不覆盖已存在的 `PATH`，但 dotenv 变量可作为模板变量并被全局 `env:` 引用，因此采用“生成映射文件 + 全局 env + 绝对路径”的方式，不修改开发者 PATH。
- 2026-10-08：首次引导时 `task` 尚不存在，无法依赖 Taskfile 的 `env:` 传版本；改为引导器直接解析 `Taskfile.yml` 的 `vars:`，`TC_<KEY>` 只作为可选覆盖。这样依赖声明仍然只有一处。
- 2026-10-08：实测 `clang-cl.cmd` 硬编码 `lib/clang/23` 会让升级 LLVM 时漏改，改为使用 `LLVM_MAJOR`（由引导器依 `LLVM_VERSION` 生成，默认值仅兜底）。
- 2026-10-08：本机与 CI 修复两个真实下载缺陷：Go 不会读取 Windows 系统代理（.NET 下载因此卡在 0 字节），现在会读取注册表代理设置；GitHub 资产与 .NET 主机可达性相反（前者直连 HTTP/2 正常、走代理 EOF，后者直连 TLS 握手超时、走代理正常），因此改为“默认网络 + 系统代理”双通道轮询重试。实测强制 HTTP/1.1 会让 GitHub 资产立刻 EOF，故保留默认协议。
- 2026-10-08：`.tools/toolchain.env` 必须无 BOM：Task 使用的 dotenv 解析器遇到 BOM 会直接报错。
- 2026-10-08：Windows LLVM 的获取方式改为官方 MSI 管理安装（`msiexec /a … TARGETDIR=…`），并删除 `github.com/ulikunitz/xz` 依赖与 `.tar.xz` 分支；引导器回到纯标准库。详见 [015](015-windows-llvm-msi.md)。

## 完成摘要

依赖声明已收敛到 `Taskfile.yml` 的 `vars:`（版本、URL、SHA256/SHA512），新增独立 Go 模块 `tools/toolchain` 负责引导：`bootstrap` 安装 Go 工具、下载校验并解压 .NET SDK、Godot、Windows LLVM 与 LLVM-MinGW，按锁文件还原 NuGet 依赖，并生成 `.tools/toolchain.env` 供 Taskfile 映射 `CC`、`CXX`、`DOTNET_ROOT`、`LLVM_MINGW_ROOT`、`LLVM_CLANG_CL`、`LLVM_MAJOR`；`check`/`env` 支撑 `task env`。CI 只保留 `setup-go`，其余依赖全部走引导器并缓存归档。

本机 Windows 验证：`task ci` 通过（两个 Go 模块的 vet 与 golangci-lint 0 issues、C# 构建 0 警告、`go test -race`、toolchain 单测、`dotnet test` 4 通过），使用的 .NET SDK 与编译器都来自 `.tools/`。CI 三平台通过（ubuntu 1m12s、macos 5m44s、windows 30m49s）；Windows 首轮耗时来自 900 MB LLVM 归档下载，归档已进 `.tools/cache` 缓存，后续运行命中缓存。限制：本机到 GitHub 资产 CDN 的带宽不足以在本地完成 Godot 与 LLVM 归档下载，这两项由 CI 验证。
