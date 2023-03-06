package action

import (
	"context"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	model2 "github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"os/exec"
	"path"
)

// RemoteAction 执行远程命令
type RemoteAction struct {
	name string
	args map[string]string
	ctx  context.Context

	actionRoot string
}

func NewRemoteAction(step model2.Step, ctx context.Context) *RemoteAction {
	return &RemoteAction{
		name: step.Uses,
		args: step.With,
		ctx:  ctx,
	}
}

func (a *RemoteAction) Pre() error {

	stack := a.ctx.Value(STACK).(map[string]interface{})

	pipelineName := stack["name"].(string)

	logger.Infof("git stack: %v", stack)

	hamsterRoot := stack["hamsterRoot"].(string)
	cloneDir := utils.RandSeq(9)
	a.actionRoot = path.Join(hamsterRoot, cloneDir)

	_ = os.MkdirAll(hamsterRoot, os.ModePerm)
	_ = os.Remove(path.Join(hamsterRoot, pipelineName))

	githubUrl := fmt.Sprintf("https://github.com/%s", a.name)

	commands := []string{"git", "clone", "--progress", githubUrl, cloneDir}
	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = hamsterRoot

	fmt.Println(a.name)
	fmt.Println(a.args)

	output, err := c.CombinedOutput()
	fmt.Println(string(output))
	return err
}

func (a *RemoteAction) Hook() (*model2.ActionResult, error) {

	stack := a.ctx.Value(STACK).(map[string]interface{})
	env, ok := stack["env"].([]string)
	if !ok {
		return nil, errors.New("env cannot be empty")
	}

	file, err := os.Open(path.Join(a.actionRoot, "action.yml"))
	if err != nil {
		return nil, err
	}
	yamlFile, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	var remoteAction model2.RemoteAction
	err = yaml.Unmarshal(yamlFile, &remoteAction)

	for envName := range remoteAction.Inputs {
		env = append(env, fmt.Sprintf("%s=%s", envName, a.args[envName]))
	}

	for index, step := range remoteAction.Runs.Steps {
		args := make([]string, 0)
		if _, err := os.Stat(path.Join(a.actionRoot, step.Run)); err == nil {
			args = append(args, step.Run)
		} else {
			stepFile := path.Join(a.actionRoot, fmt.Sprintf("step-%d", index))
			err = os.WriteFile(stepFile, []byte(step.Run), os.ModePerm)
			if err != nil {
				return nil, err
			}
			args = append(args, "-c", stepFile)
		}

		cmd := utils.NewCommand(a.ctx, step.Shell, args...)
		cmd.Dir = a.actionRoot
		cmd.Env = env
		output, _ := cmd.CombinedOutput()
		fmt.Println(string(output))
	}

	return nil, err
}

func (a *RemoteAction) Post() error {

	return os.RemoveAll(a.actionRoot)
}
