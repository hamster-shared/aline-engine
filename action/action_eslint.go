package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/tidwall/gjson"
	"os"
	"os/exec"
	path2 "path"
	"strconv"
	"strings"
)

// EslintAction 前端代码检查
type EslintAction struct {
	path   string
	ctx    context.Context
	output *output.Output
}

func NewEslintAction(step model.Step, ctx context.Context, output *output.Output) *EslintAction {
	return &EslintAction{
		path:   step.With["path"],
		ctx:    ctx,
		output: output,
	}
}

func (a *EslintAction) Pre() error {
	eslintFile := ".eslintrc.js"
	packageFile := "package.json"
	stack := a.ctx.Value(STACK).(map[string]interface{})
	workdir, ok := stack["workdir"].(string)
	if !ok {
		return errors.New("workdir is empty")
	}
	files, _ := os.ReadDir(workdir)
	eslintFileFlag := false
	packageFileFlag := false
	for _, file := range files {
		if file.Name() == eslintFile {
			eslintFileFlag = true
		}
		if file.Name() == packageFile {
			packageFileFlag = true
		}
	}
	if !eslintFileFlag {
		return errors.New(".eslintrc.js not exist")
	}

	if !packageFileFlag {
		return errors.New("package.json not exist")
	}
	packagePath := path2.Join(workdir, packageFile)
	readFile, err := os.ReadFile(packagePath)
	if err != nil {
		return err
	}
	if !gjson.GetBytes(readFile, "scripts.lint").Exists() {
		return errors.New("lint cli not exist")
	}
	return nil
}

func (a *EslintAction) Hook() (*model.ActionResult, error) {

	a.output.NewStep("eslint-check")

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
	destDir := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.CheckName, jobId, consts.EslintCheckOutputDir)
	_, err = os.Stat(destDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	dest := path2.Join(destDir, consts.EslintCheckOutputDir+consts.SuffixType)
	out, err := a.ExecuteCommand(strings.Fields("npm run lint"), workdir)
	if out == "" && err != nil {
		return nil, err
	}
	firstIndex := strings.Index(out, "\n")
	afterTheFirstLine := out[firstIndex+1:]
	secondIndex := strings.Index(afterTheFirstLine, "\n")
	afterTheSecondLine := afterTheFirstLine[secondIndex+1:]
	thirdIndex := strings.Index(afterTheSecondLine, "\n")
	afterTheThirdLine := afterTheSecondLine[thirdIndex+1:]
	fourIndex := strings.Index(afterTheThirdLine, "\n")
	create, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	_, err = create.WriteString(afterTheThirdLine[fourIndex+1:])
	if err != nil {
		return nil, err
	}
	create.Close()

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

func (a *EslintAction) Post() error {
	file, err := os.ReadFile(path2.Join(a.path, consts.EslintCheckOutputDir+consts.SuffixType))
	if err != nil {
		return errors.New("check result path is err")
	}
	var eslintCheckResultList []EslintCheckResult
	err = json.Unmarshal(file, &eslintCheckResultList)
	if err != nil {
		return errors.New("get check result is fail")
	}
	if len(eslintCheckResultList) < 1 {
		return nil
	}
	var checkResultDetailsList []model.ContractCheckResultDetails[[]model.EslintCheckReportDetails]
	for _, eslintCheckResult := range eslintCheckResultList {
		if len(eslintCheckResult.Messages) < 1 {
			continue
		}
		var checkResultDetails model.ContractCheckResultDetails[[]model.EslintCheckReportDetails]
		checkResultDetails.Name = eslintCheckResult.FilePath
		checkResultDetails.Issue = len(eslintCheckResult.Messages)
		var eslintCheckReportDetailsList []model.EslintCheckReportDetails
		for _, message := range eslintCheckResult.Messages {
			var eslintCheckReportDetails model.EslintCheckReportDetails
			eslintCheckReportDetails.Tool = message.RuleId
			eslintCheckReportDetails.Note = message.Message
			eslintCheckReportDetails.Line = strconv.Itoa(message.Line)
			eslintCheckReportDetails.Column = strconv.Itoa(message.Column)
			if message.Severity == 1 {
				eslintCheckReportDetails.Level = "error"
			} else {
				eslintCheckReportDetails.Level = "waring"
			}
			eslintCheckReportDetailsList = append(eslintCheckReportDetailsList, eslintCheckReportDetails)
		}
		checkResultDetails.Message = eslintCheckReportDetailsList
		checkResultDetailsList = append(checkResultDetailsList, checkResultDetails)
	}
	checkResult := model.NewContractCheckResult(consts.FrontEndCheckReport.Name, consts.CheckSuccess.Result, consts.FrontEndCheckReport.Tool, checkResultDetailsList)
	create, err := os.Create(path2.Join(a.path, consts.CheckResult))
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

func (a *EslintAction) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute mythril check command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	a.output.WriteCommandLine(string(out))
	if err != nil {
		a.output.WriteLine(err.Error())
	}
	return string(out), err
}

type EslintCheckResult struct {
	FilePath            string                     `json:"filePath"`
	Messages            []EslintCheckResultMessage `json:"messages"`
	SuppressedMessages  []interface{}              `json:"suppressedMessages"`
	ErrorCount          int                        `json:"errorCount"`
	FatalErrorCount     int                        `json:"fatalErrorCount"`
	WarningCount        int                        `json:"warningCount"`
	FixableErrorCount   int                        `json:"fixableErrorCount"`
	FixableWarningCount int                        `json:"fixableWarningCount"`
	UsedDeprecatedRules []interface{}              `json:"usedDeprecatedRules"`
}

type EslintCheckResultMessage struct {
	RuleId    string      `json:"ruleId"`
	Severity  int         `json:"severity"`
	Message   string      `json:"message"`
	Line      int         `json:"line"`
	Column    int         `json:"column"`
	NodeType  interface{} `json:"nodeType"`
	MessageId string      `json:"messageId"`
}
