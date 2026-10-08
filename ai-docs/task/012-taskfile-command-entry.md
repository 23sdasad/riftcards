# 012 — 统一 Taskfile 命令入口与版本锁定

- 状态：done
- 依赖：011
- 优先级：P0
- 创建 / 更新：2026-10-08 / 2026-10-08

## 目标与背景

需求方先确认可以删除根目录 `Taskfile.yml`（提交 `9c83f0b`），随后更正：Taskfile 要保留，作为 Go、.NET 和 Godot 命令的唯一跨平台入口。本任务恢复根目录 `Taskfile.yml`，把 CI 与文档切回同一入口，并锁定工具链与依赖版本，使三平台 CI 能真正跑通。

本文件按需求方要求改写：同编号的删除方案已提交为 `9c83f0b`，其行为与结论由本任务取代，历史提交仍保留。

## 必读

[测试策略](../architecture/testing.md) · [技术栈](../contracts/tech-stack.md) · [Path 与工程边界](../architecture/paths-and-boundaries.md) · [提交规范](../standards/commits.md)。

## 范围与非目标

交付：

- 恢复根目录 `Taskfile.yml`，提供 `env`、`bootstrap`、`fmt`、`fmt:check`、`lint`、`test`、`check`、`ci`、`run:server`、`run:client`、`vuln`，全部命令在三平台走 Task 内置 POSIX 解释器。
- `task env` 逐个检查工具，缺项时给出工具名、最低版本和安装入口，并返回非零退出码。
- `task ci` 是与 CI 相同的只读门禁；`run:server`、`run:client` 用前置条件保证缺工具时不启动半个进程。
- CI 接回 `task`：Go、.NET、缓存与 `setup-task` 全部按 commit SHA 固定，步骤改为 `task bootstrap`、`dotnet restore --locked-mode`、`task ci`。
- 修复 CI 首次失败：`KyleMayes/install-llvm-action@v2` 的资产表没有 LLVM 23.1.3。
- 修复 CI 第二轮失败：macOS 上 Go 按 `server/go.mod` 装 1.25.0，而 golangci-lint 2.14.0 要求 Go 1.26+，工具链自动切换下载失败；Windows 上 runner 自带 `clang-cl` 版本低于 23，与包装器强制的 `-resource-dir=<LLVM-MinGW>\lib\clang\23` 不匹配。
- 把 `server/go.mod` 的 Go 下限对齐到 1.27，并在 CI 中为 Windows 安装被锁定的 LLVM 23.1.3 压缩包。
- 修复 Windows cgo：Go 构建 `runtime/cgo` 自带 `-Werror`，`tools/llvm/clang-cl.cmd` 需要降级 `-Wunused-command-line-argument`。
- 锁定版本：`global.json`（.NET SDK）、三个 `packages.lock.json`（NuGet 与 Godot SDK 包）、`Taskfile.yml`（golangci-lint 2.14.0、govulncheck 1.1.4、最低版本）、CI（Go Task 3.54.0、LLVM-MinGW URL+SHA256）。
- 同步 README、AGENTS、`ai-docs` 架构/契约/规范文档与相关 task。

非目标：

- 不改业务代码、协议、规则或 Go/C# 检查口径。
- 不引入第二套任务器（Just、Make、npm scripts）或平台专用脚本。
- 不把 Godot 图形化测试和端到端测试放进 CI。

## 前置条件与待决策

- 本机有 Go 1.27.0、.NET SDK 10.0.401、Go Task 3.54.0、LLVM 23.1.3 和 LLVM-MinGW 20260908；Godot 不在 PATH，`task env` 如实报告。
- CI 是否需要下载完整 LLVM 发行包：已决定不下载。上游 action 无 23.1.3 资产，Linux 完整包约 2 GB；改为 Unix 使用 runner 自带 `clang` 并打印版本，Windows 下载并校验 SHA256 固定的 LLVM-MinGW，`clang-cl` 取自 runner 自带 LLVM。

## 实施步骤

1. 恢复 `Taskfile.yml`，在 Windows 上用探针确认内置解释器、任务级 `dir` 和 `preconditions` 行为。
2. 增加 `global.json` 与 `RestorePackagesWithLockFile`，生成三个 `packages.lock.json`。
3. 改写 CI workflow：按 SHA 固定 action，去掉失效的 LLVM action，接回 `task`，增加锁文件校验。
4. 修复 `clang-cl.cmd` 在 `-Werror` 下的未使用参数错误。
5. 更新 README、AGENTS、架构、契约、规范和 task 文档。
6. 本地跑 `task env`、`task ci` 和差异/链接检查，推送后确认三平台 CI。

## 预计改动

- 新增：`Taskfile.yml`、`global.json`、三个 `packages.lock.json`、`ai-docs/task/012-taskfile-command-entry.md`。
- 修改：`.github/workflows/ci.yml`、三个 `.csproj`、`server/go.mod`、`tools/llvm/clang-cl.cmd`、`tools/llvm/README.md`、`README.md`、`AGENTS.md`、`ai-docs/README.md`、`ai-docs/architecture/{conventions,testing,paths-and-boundaries}.md`、`ai-docs/contracts/{tech-stack,library-research}.md`、`ai-docs/standards/commits.md`、task 004/005/006/011/013 与 `ai-docs/task-index.md`。
- 删除：`ai-docs/task/012-remove-taskfile.md`（由本文件取代）。

## 清理与兼容例外

删除 CI 中失效的 LLVM action、浮动的 `govulncheck@latest` 与 `8.0.x` SDK 口径，以及“直接命令 + `task`”两套文档口径。`Taskfile.yml` 是唯一命令入口。

## 验收标准

- [x] 根目录 `Taskfile.yml` 提供全部命令，并在 Windows 上验证内置解释器、任务级 `dir` 与 `preconditions`。
- [x] `task env` 缺工具时给出工具名、最低版本、安装入口，并返回非零退出码。
- [x] `run:server`、`run:client`、`lint`、`test` 使用前置条件，缺工具时不启动进程。
- [x] CI 不再依赖失效的 LLVM action，`task` 入口与所有第三方 action commit 固定。
- [x] .NET SDK、NuGet、Godot SDK、Go 工具、LLVM 与 CI 专用依赖各有唯一权威锁定位置。
- [x] Go 版本下限与 golangci-lint 要求一致，CI 不再触发隐式工具链下载。
- [x] Windows 使用被锁定的 LLVM 23.1.3 驱动，与 `clang-cl.cmd` 的 `-resource-dir` 版本一致。
- [x] `task ci` 在本机通过：格式检查、lint、C# 构建、Go race 测试和 C# 单测全绿。
- [x] README、AGENTS、架构、契约与规范文档与 `Taskfile.yml` 实际入口一致。
- [x] 三平台 CI 实跑通过。

## 验证计划与结果

| 日期 | 环境 / 命令 | 预期 | 实际结果 |
| --- | --- | --- | --- |
| 2026-10-08 | Windows / `task --list` | 全部任务可见 | 通过 |
| 2026-10-08 | Windows / `task env` | 列出工具版本，缺失项含最低版本与安装入口，非零退出 | 通过：go 1.27.0、dotnet 10.0.401、clang/clang-format/clang-tidy/clangd 23.1.3、golangci-lint 2.14.0、govulncheck、task 3.54.0 正常；godot 报 MISSING，退出码 201 |
| 2026-10-08 | Windows / `task ci` | 与 CI 相同的只读门禁 | 通过：`gofmt -l` 无输出；`dotnet format --verify-no-changes` 三项通过；`go vet` 通过；golangci-lint 0 issues；C# 构建 0 警告 0 错误；`go test -race` game 与 pathutil 通过；`dotnet test` 4 通过 0 失败 |
| 2026-10-08 | Windows / LLVM-MinGW 校验 | SHA256 与 CI 锁定值一致 | 通过：`1bcf74d06b724aeecaa6412ca85f5b26fb1da770e7cdcefa9263c9c5c3ad34b6` |
| 2026-10-08 | `git diff --check`、Markdown 相对链接、workflow 静态检查 | 无空白错误、链接有效、workflow 可解析 | 通过：diff 无输出；104 个相对链接有效；actionlint 1.7.12 无告警 |
| 2026-10-08 | GitHub Actions 首轮（run 37802743366） | 三平台 `task ci` 通过 | 失败：ubuntu 通过；macOS 在 `task bootstrap` 因 Go 1.25 与 golangci-lint 要求不符、工具链下载 404 失败；Windows 在 `go test -race` 因驱动 20→`resource-dir 23` 不匹配失败。已按下述修复 |
| 2026-10-08 | 修复后端到端 | 三平台通过 | 见下一行 |
| 2026-10-08 | GitHub Actions run [37804268319](https://github.com/23sdasad/riftcards/actions/runs/37804268319)（`32262b6`） | 三平台 `task ci` 通过 | 通过：ubuntu-latest 2m53s、macos-latest 4m22s、windows-latest 6m11s 全部成功 |
| — | GitHub Actions 三平台 | `task ci` 通过 | 推送后确认 |

## 风险与回退

风险是 CI 各平台的 `clang`/`clang-cl` 版本与本地不同，或锁文件在某些 runner 上不一致。回退时保留 `Taskfile.yml` 入口，只把有问题的平台步骤改回显式命令，并在 task 中记录实际版本；不恢复第二套任务器。

## 决策与工作记录

- 2026-10-08：原方案删除 Taskfile 并切换直接命令（`9c83f0b`）。需求方更正为保留 Taskfile，本任务改写为恢复统一入口并锁定版本。
- 2026-10-08：确认 Task 3.54.0 的 `dir` 只在任务级别生效，因此 Go 与 C# 检查拆成带任务级 `dir` 的子任务（`fmt:go`、`lint:cs` 等）。
- 2026-10-08：确认 CI 首次实跑失败于 `install-llvm-action@v2` 无 23.1.3 资产；改为 Unix 用 runner `clang`、Windows 用固定 LLVM-MinGW。
- 2026-10-08：确认 Go 的 `runtime/cgo` 构建自带 `-Werror`，`clang-cl.cmd` 必须加 `-Wno-unused-command-line-argument`；该文件保持 ASCII + CRLF，避免 cmd.exe 用 OEM 代码页解析中文注释出错。
- 2026-10-08：首轮三平台 CI 暴露两个真实缺陷并修复：`server/go.mod` 的 Go 下限低于 golangci-lint 2.14.0 的要求，改为 `go 1.27.0`（与本地验证版本一致）；Windows cgo 需要驱动与 `-resource-dir` 同版本，CI 改为安装被锁定的 LLVM 23.1.3 压缩包并校验 SHA256，而不是使用 runner 自带驱动。

## 完成摘要

根目录 `Taskfile.yml` 已恢复为唯一命令入口，`task env`、`task ci` 与运行任务的前置条件在三平台使用同一份定义；CI 接回 `task`，去掉无 23.1.3 资产的 LLVM action，所有第三方 action 按 commit SHA 固定；.NET SDK、NuGet、Godot SDK、Go 版本与 Go 工具、LLVM、LLVM-MinGW 与 CI 专用依赖均已锁定到唯一权威位置。首轮三平台 CI 暴露并修复了两个真实缺陷：Go 下限与 golangci-lint 要求不一致，以及 Windows cgo 驱动与 `-resource-dir` 版本不匹配；修复后三平台 CI（run 37804268319）全部通过。本机 Windows 全量 `task ci` 通过（gofmt、dotnet format、go vet、golangci-lint 0 issues、C# 构建 0 警告、Go race 测试与 4 个 C# 单测）。限制：本机没有 Godot，Godot 相关命令未能在本机执行。
