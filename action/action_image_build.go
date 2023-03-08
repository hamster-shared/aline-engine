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

type ImageBuildAction struct {
	imageName string
	runBuild  string
	output    *output.Output
	ctx       context.Context
}

func NewImageBuildAction(step model.Step, ctx context.Context, output *output.Output) *ImageBuildAction {
	return &ImageBuildAction{
		imageName: step.With["image_name"],
		runBuild:  step.With["run_build"],
		ctx:       ctx,
		output:    output,
	}
}

func (i *ImageBuildAction) Pre() error {
	stack := i.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	i.imageName = utils.ReplaceWithParam(i.imageName, params)
	logger.Debugf("k8s build image is : %s", i.imageName)
	i.runBuild = utils.ReplaceWithParam(i.runBuild, params)
	logger.Debugf("run build is: %s", i.runBuild)
	return nil
}

func (i *ImageBuildAction) Hook() (*model.ActionResult, error) {
	stack := i.ctx.Value(STACK).(map[string]interface{})
	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("get workdir error")
	}
	if i.runBuild == "true" {
		installCmd := []string{"npm", "install"}
		_, err := i.ExecuteCommand(installCmd, workdir)
		if err != nil {
			return nil, errors.New("npm install failed")
		}
		codeBuildCmd := []string{"npm", "run", "build"}
		_, err = i.ExecuteCommand(codeBuildCmd, workdir)
		if err != nil {
			return nil, errors.New("npm run build failed")
		}
	}
	buildCommands := []string{"docker", "buildx", "build", "-t", i.imageName, "--platform=linux/amd64", "."}
	_, err := i.ExecuteCommand(buildCommands, workdir)
	if err != nil {
		return nil, errors.New("docker build image failed")
	}
	return nil, nil
}
func (i *ImageBuildAction) Post() error {
	return nil
}

func (i *ImageBuildAction) ExecuteCommand(commands []string, workdir string) (string, error) {
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
