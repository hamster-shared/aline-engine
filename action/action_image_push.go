package action

import (
	"context"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"os/exec"
	"strings"
)

type ImagePushAction struct {
	imageName string
	output    *output.Output
	ctx       context.Context
}

func NewImagePushAction(step model.Step, ctx context.Context, output *output.Output) *ImagePushAction {
	return &ImagePushAction{
		imageName: step.With["image_name"],
		ctx:       ctx,
		output:    output,
	}
}

func (i *ImagePushAction) Pre() error {
	stack := i.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	i.imageName = utils.ReplaceWithParam(i.imageName, params)
	logger.Debugf("k8s push image is : %s", i.imageName)
	return nil
}

func (i *ImagePushAction) Hook() (*model.ActionResult, error) {
	stack := i.ctx.Value(STACK).(map[string]interface{})
	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("get workdir error")
	}
	pushCommands := []string{"docker", "push", i.imageName}
	_, err := i.ExecuteCommand(pushCommands, workdir)
	if err != nil {
		return nil, errors.New("docker push image failed")
	}
	actionResult := &model.ActionResult{
		BuildData: []model.BuildInfo{
			{
				ImageName: i.imageName,
			},
		},
	}
	return actionResult, nil
}
func (i *ImagePushAction) Post() error {
	return nil
}

func (i *ImagePushAction) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(i.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute docker command: %s", strings.Join(commands, " "))
	i.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	i.output.WriteCommandLine(string(out))
	if err != nil {
		i.output.WriteLine(err.Error())
	}
	return string(out), err
}
