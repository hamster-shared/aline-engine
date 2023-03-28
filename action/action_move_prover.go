package action

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/utils"
	"io"
	"os"
	"os/exec"
	path2 "path"
	"regexp"
	"strconv"
	"strings"

	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
)

// MoveProverAction MoveProver 合约检查
type MoveProverAction struct {
	path   string
	ctx    context.Context
	output *output.Output
}

func NewMoveProverAction(step model.Step, ctx context.Context, output *output.Output) *MoveProverAction {
	return &MoveProverAction{
		path:   step.With["path"],
		ctx:    ctx,
		output: output,
	}
}

func (a *MoveProverAction) Pre() error {
	return nil
}

func (a *MoveProverAction) Hook() (*model.ActionResult, error) {

	a.output.NewStep("MoveProver")

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
	absPathList = utils.GetSuffixFiles(basePath, consts.MoveFileSuffix, absPathList)
	destDir := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.CheckName, jobId, consts.MoveProveCheckOutputDir)
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
		redundantPath, err := utils.GetRedundantPath(workdir, path)
		commandTemplate := consts.MoveProveCheck
		//获取--named-addresses
		namedAddress, err := utils.GetDefaultNamedAddress(path2.Join(workdir, "Move.toml"))
		if err != nil {
			return nil, err
		}
		command := fmt.Sprintf(commandTemplate, workdir, path2.Join("/tmp", redundantPath))
		if namedAddress != "" {
			command = command + " --named-addresses " + namedAddress
		}
		fields := strings.Fields(command)
		out, err := a.ExecuteCommand(fields, workdir)
		if out == "" && err != nil {
			return nil, err
		}
		compile, err := regexp.Compile(`\[.{1,2}m`)
		if err != nil {
			logger.Errorf("regexp err %s", err)
			return nil, err
		}
		replaceAfterString := compile.ReplaceAllString(out, "")
		create, err := os.Create(dest)
		if err != nil {
			return nil, err
		}
		_, err = create.WriteString(replaceAfterString)
		if err != nil {
			return nil, err
		}
		create.Close()
	}

	a.path = destDir
	id, err := strconv.Atoi(jobId)
	if err != nil {
		return nil, err
	}
	actionResult := model.ActionResult{
		Artifactorys: nil,
		Reports: []model.Report{
			{
				Id:   id,
				Url:  "",
				Type: 2,
			},
		},
	}
	return &actionResult, err
}

func (a *MoveProverAction) Post() error {
	open, err := os.Open(a.path)
	if err != nil {
		return err
	}
	fileInfo, err := open.Stat()
	if err != nil {
		return err
	}
	isDir := fileInfo.IsDir()
	if !isDir {
		return errors.New("check result path is err")
	}
	fileInfos, err := open.Readdir(-1)
	if err != nil {
		return err
	}
	successFlag := true
	var checkResultDetailsList []model.ContractCheckResultDetails[[]model.ContractStyleGuideValidationsReportDetails]
	for _, info := range fileInfos {
		path := path2.Join(a.path, info.Name())
		var styleGuideValidationsReportDetailsList []model.ContractStyleGuideValidationsReportDetails
		file, err := os.Open(path)
		if err != nil {
			return errors.New("file open fail")
		}
		defer file.Close()

		line := bufio.NewReader(file)
		for {
			content, _, err := line.ReadLine()
			if err == io.EOF {
				break
			}
			s := string(content)
			if len(s) > 5 && strings.Contains(s, "error: ") {
				var styleGuideValidationsReportDetails model.ContractStyleGuideValidationsReportDetails
				styleGuideValidationsReportDetails.Tool = consts.MoveProve
				styleGuideValidationsReportDetails.Note = s[6:]
				pathContent, _, err := line.ReadLine()
				if err == io.EOF {
					break
				}
				pathString := string(pathContent)
				if strings.Contains(pathString, "┌─") && strings.Contains(pathString, ":") {
					split := strings.Split(pathString, "┌─")
					if len(split) < 2 {
						continue
					}
					fileSplit := strings.Split(split[1], ":")
					if len(fileSplit) < 3 {
						continue
					}
					styleGuideValidationsReportDetails.OriginalText = fileSplit[0]
					styleGuideValidationsReportDetails.Line = fileSplit[1]
					styleGuideValidationsReportDetails.Column = fileSplit[2]
				}
				styleGuideValidationsReportDetailsList = append(styleGuideValidationsReportDetailsList, styleGuideValidationsReportDetails)
			}
		}
		if len(styleGuideValidationsReportDetailsList) > 0 {
			successFlag = false
		}
		details := model.NewContractCheckResultDetails(strings.Replace(info.Name(), consts.SuffixType, consts.MoveFileSuffix, 1), len(styleGuideValidationsReportDetailsList), styleGuideValidationsReportDetailsList)
		checkResultDetailsList = append(checkResultDetailsList, details)
	}
	var result string
	if successFlag {
		result = consts.CheckSuccess.Result
	} else {
		result = consts.CheckFail.Result
	}
	checkResult := model.NewContractCheckResult(consts.FormalSpecificationAndVerificationReport.Name, result, consts.FormalSpecificationAndVerificationReport.Tool, checkResultDetailsList)
	create, err := os.Create(path2.Join(a.path, consts.CheckResult))
	fmt.Println(checkResult)
	if err != nil {
		return err
	}
	marshal, err := json.Marshal(checkResult)
	if err != nil {
		return err
	}
	_, err = create.WriteString(string(marshal))
	if err != nil {
		return err
	}
	create.Close()
	return nil
}

func (a *MoveProverAction) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute move prove check command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	a.output.WriteCommandLine(string(out))
	if err != nil {
		a.output.WriteLine(err.Error())
	}
	return string(out), err
}
