package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	path2 "path"
	"strconv"
	"strings"

	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
)

// MoveLint MoveLint Sui contract check
type MoveLint struct {
	path   string
	ctx    context.Context
	output *output.Output
}

func NewMoveLint(step model.Step, ctx context.Context, output *output.Output) *MoveLint {
	return &MoveLint{
		path:   step.With["path"],
		ctx:    ctx,
		output: output,
	}
}

func (m *MoveLint) Pre() error {
	return nil
}

func (m *MoveLint) Hook() (*model.ActionResult, error) {

	stack := m.ctx.Value(STACK).(map[string]interface{})

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

	codePath := path2.Join(workdir, m.path)
	destDir := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.CheckName, jobId, consts.MoveLintCheckOutputDir)
	_, err = os.Stat(destDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	commandTemplate := consts.MoveLintCheck
	command := fmt.Sprintf(commandTemplate, codePath)
	fields := strings.Fields(command)
	out, err := m.ExecuteCommand(fields, workdir)
	if out == "" && err != nil {
		return nil, err
	}
	if strings.Contains(out, "Error {") {
		return nil, errors.New(out)
	}
	dest := path2.Join(destDir, consts.MoveLintCheckOutputDir+consts.SuffixType)
	create, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	_, err = create.WriteString(out)
	if err != nil {
		return nil, err
	}
	create.Close()

	m.path = destDir
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

func (m *MoveLint) Post() error {
	dest := path2.Join(m.path, consts.MoveLintCheckOutputDir+consts.SuffixType)
	fileByte, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	successFlag := true
	var checkResultDetailsList []model.ContractCheckResultDetails[[]model.ContractStyleGuideValidationsReportDetails]
	var total int
	startIndex := strings.Index(string(fileByte), "[{")
	var moveLintJsonList []MoveLintJson
	var resultString string
	if startIndex != -1 {
		resultString = string(fileByte)[startIndex:]
		err := json.Unmarshal([]byte(resultString), &moveLintJsonList)
		if err != nil {
			return err
		}
	}

	fileToMoveLintJsonMap := make(map[string][]MoveLintJson)
	for _, moveLintJson := range moveLintJsonList {
		fileToMoveLintJsonMap[moveLintJson.File] = append(fileToMoveLintJsonMap[moveLintJson.File], moveLintJson)
	}

	for file, moveLintJsons := range fileToMoveLintJsonMap {
		var suiCheckReportDetailsList []model.ContractStyleGuideValidationsReportDetails
		for _, moveLintJson := range moveLintJsons {
			var suiCheckReportDetails model.ContractStyleGuideValidationsReportDetails
			suiCheckReportDetails.Level = moveLintJson.Level
			suiCheckReportDetails.Line = strconv.Itoa(moveLintJson.Lines[0])
			suiCheckReportDetails.Note = moveLintJson.Title + ": " + moveLintJson.Verbose
			suiCheckReportDetailsList = append(suiCheckReportDetailsList, suiCheckReportDetails)
			successFlag = false
		}
		contractCheckResultDetails := model.NewContractCheckResultDetails(file, len(suiCheckReportDetailsList), suiCheckReportDetailsList)
		total = total + contractCheckResultDetails.Issue
		checkResultDetailsList = append(checkResultDetailsList, contractCheckResultDetails)
	}

	var result string
	if successFlag {
		result = consts.CheckSuccess.Result
	} else {
		result = consts.CheckFail.Result
	}
	checkResult := model.NewContractCheckResult(consts.SuiContractSecurityAnalysisReport.Name, result, consts.SuiContractSecurityAnalysisReport.Tool, checkResultDetailsList, total)
	create, err := os.Create(path2.Join(m.path, consts.CheckResult))
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

func (m *MoveLint) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(m.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute move-lint check command: %s", strings.Join(commands, " "))
	m.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	m.output.WriteCommandLine(string(out))
	if err != nil {
		m.output.WriteLine(err.Error())
	}
	return string(out), err
}

type MoveLintJson struct {
	No          int    `json:"no"`
	Wiki        string `json:"wiki"`
	Title       string `json:"title"`
	Verbose     string `json:"verbose"`
	Level       string `json:"level"`
	Description string `json:"description"`
	File        string `json:"file"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Lines       []int  `json:"lines"`
}
