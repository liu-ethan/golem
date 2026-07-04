package headless

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tencent-docs/golem/internal/agent"
)

// Run 执行 headless 单次对话；query 为 positional 参数合并后的用户输入。
func Run(ctx context.Context, ag *agent.Agent, query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("query is required")
	}

	handler := func(evt agent.Event) {
		switch evt.Type {
		case agent.EventTextDelta:
			fmt.Fprint(os.Stdout, evt.Text)
		case agent.EventError:
			if evt.Err != nil {
				fmt.Fprintf(os.Stderr, "\nError: %v\n", evt.Err)
			}
		}
	}

	_, err := ag.HandleInput(ctx, query, handler)
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return err
	}
	ag.OnSessionEnd()
	return nil
}
