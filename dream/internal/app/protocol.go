package app

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	taskDonePattern = regexp.MustCompile(`(?m)^TASK_DONE:\s*(.+?)\s*$`)
	nextTaskPattern = regexp.MustCompile(`(?m)^NEXT_TASK:\s*(.+?)\s*$`)
	allDonePattern  = regexp.MustCompile(`(?m)^ALL_TASKS_DONE\s*$`)
)

type RoundOutcome struct {
	TaskDone string
	NextTask string
	AllDone  bool
}

func BuildRoundPrompt(docPath, projectRoot string, round int) string {
	return fmt.Sprintf(`你正在由 dream 驱动执行一个顺序任务工作流。

工作流规则：
1. 先阅读方案/提示词文档：%s
2. 项目根目录是：%s
3. 阅读代码和文档后，只执行当前最应该做的一个任务，不要同时推进下一个任务。
4. 完成该任务后，必须直接更新同一份文档，写清已完成内容、当前进度和下一步任务。
5. 你可以执行必要的构建、测试和修复，但范围只限于本轮选中的那一个任务。
6. 不要依赖之前的会话上下文；当前轮次是一个新的 session，需要只基于文档和代码重新判断。

最终输出要求：
- 如果本轮完成了一个任务，而且文档里还有后续任务，最终输出末尾必须包含两行纯文本标记：
TASK_DONE: <一句话描述本轮完成内容>
NEXT_TASK: <一句话描述下一步任务>
- 如果文档中的所有任务都已经完成，最终输出末尾必须包含一行纯文本标记：
ALL_TASKS_DONE
- 这些标记不要放进代码块。

当前轮次：%d
`, docPath, projectRoot, round)
}

func BuildResumePrompt(round int) string {
	return fmt.Sprintf(`继续当前已中断的这一轮任务。

要求：
1. 继续当前轮次，不要切换到下一个任务。
2. 仍然必须遵守本次工作流里既有的 TASK_DONE / NEXT_TASK / ALL_TASKS_DONE 输出协议。
3. 完成当前轮次后再结束。

当前轮次：%d
`, round)
}

func ParseRoundOutcome(lastMessage string) (RoundOutcome, error) {
	outcome := RoundOutcome{
		AllDone: allDonePattern.MatchString(lastMessage),
	}
	if outcome.AllDone {
		return outcome, nil
	}

	taskDone := taskDonePattern.FindStringSubmatch(lastMessage)
	if len(taskDone) != 2 {
		return RoundOutcome{}, fmt.Errorf("missing TASK_DONE marker")
	}
	nextTask := nextTaskPattern.FindStringSubmatch(lastMessage)
	if len(nextTask) != 2 {
		return RoundOutcome{}, fmt.Errorf("missing NEXT_TASK marker")
	}

	outcome.TaskDone = strings.TrimSpace(taskDone[1])
	outcome.NextTask = strings.TrimSpace(nextTask[1])
	return outcome, nil
}
