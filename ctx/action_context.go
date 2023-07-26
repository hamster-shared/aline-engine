package ctx

import (
	"context"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
)

const STACK = "stack"

type ActionContext struct {
	step   model.Step
	ctx    context.Context
	output *output.Output
}

func NewActionContext(step model.Step, ctx context.Context, output *output.Output) ActionContext {
	return ActionContext{
		step:   step,
		ctx:    ctx,
		output: output,
	}
}

func (a *ActionContext) GetStack() map[string]interface{} {
	return a.ctx.Value(STACK).(map[string]interface{})
}

func (a *ActionContext) GetStackValue(key string) any {
	return a.GetStack()[key]
}

func (a *ActionContext) GetWorkdir() string {
	return a.GetStack()["workdir"].(string)
}

func (a *ActionContext) GetUserId() uint {
	return a.GetStack()["userId"].(uint)
}

func (a *ActionContext) WriteLine(content string) {
	if a.output != nil {
		a.output.WriteLine(content)
	}
}

func (a *ActionContext) GetStepWith(key string) string {
	return a.step.With[key]
}

func (a *ActionContext) GetParameters() map[string]string {
	return a.GetStackValue("parameter").(map[string]string)
}
