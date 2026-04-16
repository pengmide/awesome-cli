package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ReadmeFilename  = "README.md"
	FeatureFilename = "feature-foundation.md"
)

type Result struct {
	Dir         string
	ReadmePath  string
	FeaturePath string
}

func Create(dir string) (Result, error) {
	if strings.TrimSpace(dir) == "" {
		return Result{}, fmt.Errorf("missing template directory")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return Result{}, err
	}
	absDir = filepath.Clean(absDir)

	if err := ensureWritableDir(absDir); err != nil {
		return Result{}, err
	}

	result := Result{
		Dir:         absDir,
		ReadmePath:  filepath.Join(absDir, ReadmeFilename),
		FeaturePath: filepath.Join(absDir, FeatureFilename),
	}

	if err := os.WriteFile(result.ReadmePath, []byte(readmeContent()), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(result.FeaturePath, []byte(featureContent()), 0o644); err != nil {
		return Result{}, err
	}
	return result, nil
}

func ensureWritableDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case err == nil:
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("%s is not empty", dir)
		}
		return nil
	case !os.IsNotExist(err):
		return err
	}

	return os.MkdirAll(dir, 0o755)
}

func readmeContent() string {
	return `# 工作计划

## 背景

这一组文档用于把当前长期任务拆成可由 ` + "`dream`" + ` 按轮次持续推进的细任务。

总文档负责统一前提、功能点排期和子文档导航；子文档负责单个功能点的任务拆分、进度记录和下一步动作。

## 使用方式

- 先阅读本页，确认当前功能点、完成进度和对应文档。
- 再进入具体子文档，按“细任务拆分”逐项推进。
- 首次执行 ` + "`dream run README.md`" + ` 时，` + "`dream`" + ` 会自动在文档靠前位置补写 ` + "`PROJECT_ROOT=...`" + `。
- 每次更新子文档进度时，都要同步回写本页“文档总表”中的完成进度。

## 文档总表

| 功能点     | 任务数 | 完成进度 | 当前排期 | 文档                                           |
| ---------- | -----: | -------: | -------- | ---------------------------------------------- |
| 基础能力收口 |      3 |    0 / 3 | P0       | [feature-foundation.md](./feature-foundation.md) |

## 当前总顺序

1. ` + "`基础能力收口`" + `

## 记录要求

- 每完成一个任务，至少记录修改的文件、已完成内容、验证方式、当前进度和下一步任务。
- 子文档的完成记录应保持到下一轮仍能直接判断当前状态，避免只留下模糊结论。
- 如果后续新增功能点文档，需要同步补到本页“文档总表”和“当前总顺序”。

## 说明

- 这些文档的目标是让 ` + "`dream`" + ` 和工程师都能继续推进，而不是只做背景说明。
- 当前模板默认只生成一个示例功能点文档；你可以按项目需要继续复制扩展。
`
}

func featureContent() string {
	return `# 基础能力收口计划

> 维护要求：更新本文档任务进度时，需同步更新 ` + "`README.md`" + ` 中“文档总表”章节的“完成进度”列。
> 规范要求：本文档应持续保持“当前判断 / 细任务拆分 / 完成记录”可供 ` + "`dream`" + ` 直接续跑。

## 范围

- 待补充当前功能点涉及的模块、目录或流程。
- 建议把范围写到可以直接定位文件和责任边界的粒度。

## 当前判断

- 待补充当前功能点的重要性、阻塞点和为什么要先做这一组任务。
- 如果已有明确依赖、风险或阶段目标，也在这里先写清楚。

## 细任务拆分

| ID | 子任务           | 主要文件 | 需要补齐的内容 | 优先级 | 当前状态 |
| -- | ---------------- | -------- | -------------- | ------ | -------- |
| F1 | 待补充首个子任务 | ` + "`待补充`" + ` | 待补充         | P0     | 未开始   |
| F2 | 待补充第二个子任务 | ` + "`待补充`" + ` | 待补充         | P1     | 未开始   |
| F3 | 待补充第三个子任务 | ` + "`待补充`" + ` | 待补充         | P1     | 未开始   |

## 推荐执行顺序

1. ` + "`F1`" + `
2. ` + "`F2`" + `
3. ` + "`F3`" + `

## 依赖与注意事项

- 待补充当前功能点依赖的前置能力、外部条件或并行限制。
- 如果某些任务必须串行推进，也在这里明确写出。

## 完成记录

### 记录模板

- 完成时间：` + "`YYYY-MM-DD`" + `
- 已完成任务：` + "`Fx 任务名`" + `
- 修改的文件：` + "`path/to/file`" + `
- 已完成内容：待补充
- 验证方式：` + "`待补充命令或检查步骤`" + `
- 当前进度：` + "`0 / 3`" + `
- 下一步任务：` + "`Fx ...`" + `
`
}
