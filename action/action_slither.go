package action

import (
	"context"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"os"
	"os/exec"
	path2 "path"
	"strings"
)

// SlitherAction slither合约检查
type SlitherAction struct {
	path   string
	ctx    context.Context
	output *output.Output
}

func NewSlitherAction(step model.Step, ctx context.Context, output *output.Output) *SlitherAction {
	return &SlitherAction{
		path:   step.With["path"],
		ctx:    ctx,
		output: output,
	}
}

func (a *SlitherAction) Pre() error {
	return nil
}

func (a *SlitherAction) Hook() (*model.ActionResult, error) {

	// a.output.NewStep("slither-check")

	stack := a.ctx.Value(STACK).(map[string]interface{})

	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("workdir is empty")
	}
	jobName, ok := stack["name"].(string)
	if !ok {
		return nil, errors.New("get job name error")
	}
	jobId, ok := stack["id"].(string)
	if !ok {
		return nil, errors.New("get job id error")
	}
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Errorf("Failed to get home directory, the file will be saved to the current directory, err is %s", err.Error())
		userHomeDir = "."
	}

	var absPathList []string
	basePath := path2.Join(workdir, a.path)
	absPathList = utils.GetSuffixFiles(basePath, consts.SolFileSuffix, absPathList)
	destDir := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.CheckName, jobId, consts.SlitherCheckOutputDir)
	_, err = os.Stat(destDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	for _, path := range absPathList {
		_, filenameOnly := utils.GetFilenameWithSuffixAndFilenameOnly(path)
		dest := path2.Join(destDir, filenameOnly+consts.SuffixType)
		redundantPath, err := utils.GetRedundantPath(basePath, path)
		if err != nil {
			return nil, err
		}
		commandTemplate := consts.SlitherCheck
		command := fmt.Sprintf(commandTemplate, basePath, redundantPath)
		fields := strings.Fields(command)
		out, err := a.ExecuteCommand(fields, workdir)
		if err != nil {
			return nil, err
		}
		create, err := os.Create(dest)
		if err != nil {
			return nil, err
		}
		_, err = create.WriteString(out)
		if err != nil {
			return nil, err
		}
		create.Close()
	}
	return nil, err
}

func (a *SlitherAction) Post() error {
	return nil
}

func (a *SlitherAction) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute slither check command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	a.output.WriteCommandLine(string(out))
	if err != nil {
		a.output.WriteLine(err.Error())
	}
	return string(out), err
}
