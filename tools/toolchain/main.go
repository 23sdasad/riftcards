package main

import (
	"fmt"
	"os"
)

const usage = `riftcards 依赖引导器

用法：
  go -C tools/toolchain run . bootstrap [--without dotnet,godot,llvm,llvm-mingw,gotools] [--force]
  go -C tools/toolchain run . check
  go -C tools/toolchain run . env

说明：
  bootstrap  按根目录 Taskfile.yml 声明的版本安装 Go 工具、下载并校验 .NET SDK、
             Godot、Windows LLVM 与 LLVM-MinGW，并按锁文件还原 NuGet 依赖；
             所有内容位于仓库内 .tools/。
  check      检查依赖是否存在并满足锁定版本，缺失项给出安装入口。
  env        打印生成的 .tools/toolchain.env 映射文件。

依赖版本只由 Taskfile.yml 通过 TC_* 环境变量传入，本工具不硬编码版本。
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "bootstrap":
		opts, parseErr := parseBootstrapArgs(os.Args[2:])
		if parseErr != nil {
			err = parseErr
			break
		}
		err = runBootstrap(opts)
	case "check":
		err = runCheck()
	case "env":
		err = runEnv()
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		err = fmt.Errorf("未知子命令 %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolchain: %v\n", err)
		os.Exit(1)
	}
}

func runEnv() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	entries, err := readEnvFile(cfg)
	if err != nil {
		return fmt.Errorf("读取 %s 失败（先运行 bootstrap）：%w", relToRepo(envFilePath(cfg)), err)
	}
	fmt.Print(renderEnvFile(entries))
	return nil
}
