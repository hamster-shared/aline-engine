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
	"regexp"
	"strings"
)

type AptosBuildAction struct {
	aptosParam string
	output     *output.Output
	ctx        context.Context
}

func NewAptosBuildAction(step model.Step, ctx context.Context, output *output.Output) *AptosBuildAction {
	return &AptosBuildAction{
		aptosParam: step.With["aptos_param"],
		ctx:        ctx,
		output:     output,
	}
}

func (a *AptosBuildAction) Pre() error {
	stack := a.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	a.aptosParam = utils.ReplaceWithParam(a.aptosParam, params)
	logger.Debugf("aptos param is : %s", a.aptosParam)
	return nil
}

func (a *AptosBuildAction) Hook() (*model.ActionResult, error) {
	stack := a.ctx.Value(STACK).(map[string]interface{})
	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("get workdir error")
	}
	buildCommands := []string{"/usr/local/bin/aptos", "move", "compile", "--save-metadata", "--named-addresses", a.aptosParam}
	val, ok := stack["withEnv"]
	if ok {
		precommand := val.([]string)
		shellCommand := make([]string, len(buildCommands))
		copy(shellCommand, buildCommands)
		buildCommands = append([]string{}, precommand...)
		buildCommands = append(buildCommands, shellCommand...)
	}
	out, err := a.ExecuteCommand(buildCommands, workdir)
	logger.Debugf("aptos exec command success")
	if err != nil {
		logger.Errorf("exec aptos command failed:%s", err)
		return nil, errors.New("docker build image failed")
	}
	logger.Infof("aptos exec command result %s", out)
	handleData, moduleName := a.handleBuildOutData(out)
	sequenceData := model.BuildSequence{
		SequenceDada: handleData,
		Name:         moduleName,
	}
	actionResult := &model.ActionResult{}
	actionResult.BuildSequence = sequenceData
	return actionResult, nil
}
func (a *AptosBuildAction) Post() error {
	return nil
}

func (a *AptosBuildAction) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute docker command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	a.output.WriteCommandLine(string(out))
	if err != nil {
		a.output.WriteLine(err.Error())
	}
	return string(out), err
}

func (a *AptosBuildAction) handleBuildOutData(output string) ([]string, string) {
	pattern := `(?s)"Result":\s*\[.*?\]`
	re := regexp.MustCompile(pattern)
	match := re.FindString(output)
	resultPattern := `"[a-zA-Z0-9]+::[a-zA-Z_]+"`
	filePattern := regexp.MustCompile(`::(.+?)\"`)
	reResult := regexp.MustCompile(resultPattern)
	matches := reResult.FindAllString(match, -1)
	var data []string
	var moduleName string
	for _, selectedResult := range matches {
		matchFile := filePattern.FindStringSubmatch(selectedResult)
		if len(matchFile) >= 2 {
			value := matchFile[1]
			data = append(data, value)
		}
	}
	modulePattern := regexp.MustCompile(`BUILDING\s+(.+)`)
	moduleMatch := modulePattern.FindStringSubmatch(output)

	if len(moduleMatch) >= 2 {
		moduleName = moduleMatch[1]
	}
	return data, moduleName
}
