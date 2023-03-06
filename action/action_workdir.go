package action

import (
	"context"
	"fmt"
	"os"
	"strings"

	model2 "github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
)

type WorkdirAction struct {
	workdir string
	output  *output.Output
	ctx     context.Context
}

func NewWorkdirAction(step model2.Step, ctx context.Context, output *output.Output) *WorkdirAction {

	stack := ctx.Value(STACK).(map[string]interface{})
	env, ok := stack["env"].([]string)
	workdir := step.With["workdir"]
	if ok {
		workdir = envRender(workdir, append(env, os.Environ()...))
	} else {
		workdir = envRender(workdir, os.Environ())
	}

	return &WorkdirAction{
		ctx:     ctx,
		output:  output,
		workdir: workdir,
	}
}

func envRender(str string, envs []string) string {
	if str == "" {
		return str
	}

	for _, env := range envs {
		key := fmt.Sprintf("$%s", strings.Split(env, "=")[0])
		val := strings.Split(env, "=")[1]
		if strings.Contains(str, key) {
			str = strings.ReplaceAll(str, key, val)
		}
	}
	return str
}

func (a *WorkdirAction) Pre() error {
	return nil
}

// Hook 执行
func (a *WorkdirAction) Hook() (*model2.ActionResult, error) {
	_, err := os.Stat(a.workdir)
	if err != nil {
		err = os.MkdirAll(a.workdir, os.ModePerm)
		if err != nil {
			return nil, err
		}

	}

	stack := a.ctx.Value(STACK).(map[string]interface{})
	stack["workdir"] = a.workdir

	return nil, nil
}

// Post 执行后清理 (无论执行是否成功，都应该有 Post 的清理)
func (a *WorkdirAction) Post() error {
	return nil
}
