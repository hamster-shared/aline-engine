package action

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"io"
	"os"
	"os/exec"
	path2 "path"
	"regexp"
	"strconv"
	"strings"
)

// EthGasReporterAction EthGasReporter合约检查
type EthGasReporterAction struct {
	path        string
	solcVersion string
	ctx         context.Context
	output      *output.Output
}

func NewEthGasReporterAction(step model.Step, ctx context.Context, output *output.Output) *EthGasReporterAction {
	return &EthGasReporterAction{
		solcVersion: step.With["solc-version"],
		ctx:         ctx,
		output:      output,
	}
}

func (a *EthGasReporterAction) Pre() error {
	return nil
}

func (a *EthGasReporterAction) Hook() (*model.ActionResult, error) {

	// a.output.NewStep("eth-gas-reporter")

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

	destDir := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.CheckName, jobId, consts.EthGasReporterDir)
	_, err = os.Stat(destDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	dest := path2.Join(destDir, consts.EthGasReporterDir+consts.SuffixType)
	fields := strings.Fields(consts.EthGasReporterTruffle)
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

func (a *EthGasReporterAction) Post() error {
	open, err := os.Open(path2.Join(a.path, consts.EthGasReporterDir+consts.SuffixType))
	if err != nil {
		return err
	}
	successFlag := true
	defer open.Close()
	line := bufio.NewReader(open)
	var checkResultDetailsList []model.ContractCheckResultDetails[json.RawMessage]
	var unitTestResult model.ContractCheckResultDetails[json.RawMessage]
	var unitTestResultList []model.UnitTestResult
	unitTestResult.Name = consts.UnitTestResult

	var issuesInfo model.ContractCheckResultDetails[json.RawMessage]
	var issuesInfoList []model.IssuesInfo
	issuesInfo.Name = consts.IssuesInfo

	var gasUsageForMethods model.ContractCheckResultDetails[json.RawMessage]
	var gasUsageForMethodsList []model.GasUsageForMethods
	gasUsageForMethods.Name = consts.GasUsageForMethods

	var gasUsageForDeployments model.ContractCheckResultDetails[json.RawMessage]
	var gasUsageForDeploymentsList []model.GasUsageForDeployments
	gasUsageForDeployments.Name = consts.GasUsageForDeployments
	issues := 0
	var solcVersion string
	var gasLimit string
	var previousLineContent string
	r := regexp.MustCompile(`^\d+\)`)
	for {
		content, _, err := line.ReadLine()
		if err == io.EOF {
			break
		}

		s := strings.TrimLeft(string(content), " ")
		if strings.TrimSpace(s) == "" {
			continue
		}
		//Unit Test Result
		if strings.HasPrefix(s, "Contract: ") || strings.HasPrefix(previousLineContent, "Contract: ") {
			var testResultList []model.TestResult
			var unitTestResult model.UnitTestResult
			if previousLineContent == "" {
				unitTestResult.ContractName = strings.TrimSpace(strings.ReplaceAll(s, "Contract: ", ""))
			} else {
				unitTestResult.ContractName = strings.TrimSpace(strings.ReplaceAll(previousLineContent, "Contract: ", ""))
				var testResult model.TestResult
				if len(s) > 8 && r.MatchString(s[:8]) {
					issues++
					successFlag = false
					index := strings.Index(s, ")")
					testResult.Result = 0
					testResult.UnitTestTitle = strings.TrimSpace(s[index+1:])
					testResultList = append(testResultList, testResult)
				}
				if strings.Contains(s, "✓") {
					index := strings.Index(s, "✓")
					testResult.Result = 1
					testResult.UnitTestTitle = strings.TrimSpace(s[index+3:])
					testResultList = append(testResultList, testResult)
				}
			}

			previousLineContent = ""
			for {
				unitTestResultContent, _, err := line.ReadLine()
				if err == io.EOF {
					break
				}
				s := strings.TrimSpace(string(unitTestResultContent))
				if s == "" {
					continue
				}
				if strings.HasPrefix(s, "Contract: ") {
					previousLineContent = s
					break
				}
				if strings.Contains(s, "·--------") {
					break
				}
				var testResult model.TestResult
				if len(s) > 8 && r.MatchString(s[:8]) {
					issues++
					successFlag = false
					index := strings.Index(s, ")")
					testResult.Result = 0
					testResult.UnitTestTitle = strings.TrimSpace(s[index+1:])
					testResultList = append(testResultList, testResult)
				}
				if strings.Contains(s, "✓") {
					index := strings.Index(s, "✓")
					testResult.Result = 1
					testResult.UnitTestTitle = strings.TrimSpace(s[index+3:])
					testResultList = append(testResultList, testResult)
				}
			}
			unitTestResult.TestResultList = testResultList
			unitTestResultList = append(unitTestResultList, unitTestResult)
			continue
		}

		//Gas Usage for Methods And Gas Usage for Deployments
		if strings.Contains(s, "|") && strings.Index(s, "|") == 0 {
			replaceAll := strings.ReplaceAll(s, "|", "")
			replaceAll = strings.ReplaceAll(replaceAll, "│", "")
			split := strings.Split(replaceAll, "·")
			if strings.Contains(strings.TrimSpace(split[0]), "Solc version: ") {
				solcVersion = strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(split[0]), "Solc version: ", ""))
				gasLimit = strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(split[3]), "Block limit: ", ""))
			} else if strings.Contains(strings.TrimSpace(split[0]), "Methods") {
				for {
					gasUsageForMethodsContent, _, err := line.ReadLine()
					if err == io.EOF {
						break
					}
					s := string(gasUsageForMethodsContent)
					if strings.Contains(s, "Deployments") && strings.Contains(s, "% of limit") {
						break
					}
					if strings.Contains(s, "|") && strings.Index(s, "|") == 0 {
						replaceAll := strings.ReplaceAll(s, "|", "")
						replaceAll = strings.ReplaceAll(replaceAll, "│", "")
						methodsInfo := strings.Split(replaceAll, "·")
						if strings.Contains(methodsInfo[2], "Min") {
							continue
						}
						var gasUsageForMethods model.GasUsageForMethods
						gasUsageForMethods.ContractName = strings.TrimSpace(methodsInfo[0])
						gasUsageForMethods.Method = strings.TrimSpace(methodsInfo[1])
						gasUsageForMethods.Min = strings.TrimSpace(methodsInfo[2])
						gasUsageForMethods.Max = strings.TrimSpace(methodsInfo[3])
						gasUsageForMethods.Avg = strings.TrimSpace(methodsInfo[4])
						gasUsageForMethods.Calls = strings.TrimSpace(methodsInfo[5])
						gasUsageForMethodsList = append(gasUsageForMethodsList, gasUsageForMethods)
					}
				}
			} else {
				replaceAll := strings.ReplaceAll(s, "|", "")
				replaceAll = strings.ReplaceAll(replaceAll, "│", "")
				deploymentsInfo := strings.Split(replaceAll, "·")
				if len(deploymentsInfo) < 5 {
					continue
				}
				var gasUsageForDeployments model.GasUsageForDeployments
				gasUsageForDeployments.ContractName = strings.TrimSpace(deploymentsInfo[0])
				gasUsageForDeployments.Min = strings.TrimSpace(deploymentsInfo[1])
				gasUsageForDeployments.Max = strings.TrimSpace(deploymentsInfo[2])
				gasUsageForDeployments.Avg = strings.TrimSpace(deploymentsInfo[3])
				gasUsageForDeployments.Limit = strings.TrimSpace(deploymentsInfo[4])
				gasUsageForDeploymentsList = append(gasUsageForDeploymentsList, gasUsageForDeployments)
			}
			continue
		}

		//Issues  Info
		if (len(s) > 5 && r.MatchString(s[:5])) || previousLineContent != "" {
			var issuesInfoStringList []string
			if previousLineContent == "" {
				previousLineContent = s
			} else {
				issuesInfoStringList = append(issuesInfoStringList, s)
			}
			i := strings.Index(previousLineContent, ":")
			var issuesInfo model.IssuesInfo
			issuesInfo.ContractName = previousLineContent[i+2:]
			previousLineContent = ""
			for {
				infoContent, _, err := line.ReadLine()
				if err == io.EOF {
					break
				}
				s := strings.TrimSpace(string(infoContent))
				if s == "" {
					continue
				}
				if len(s) > 5 && r.MatchString(s[:5]) {
					previousLineContent = s
					break
				}
				issuesInfoStringList = append(issuesInfoStringList, s)
			}
			issuesInfo.IssuesInfo = issuesInfoStringList
			issuesInfoList = append(issuesInfoList, issuesInfo)
			continue
		}
	}
	var total int
	unitTestResult.Issue = issues
	unitTestResult.GasLimit = gasLimit
	unitTestResultMarshal, err := json.Marshal(unitTestResultList)
	if err != nil {
		return err
	}
	unitTestResult.Message = unitTestResultMarshal
	total = total + unitTestResult.Issue
	checkResultDetailsList = append(checkResultDetailsList, unitTestResult)

	issuesInfo.Issue = issues
	issuesInfo.GasLimit = gasLimit
	issuesInfoMarshal, err := json.Marshal(issuesInfoList)
	if err != nil {
		return err
	}
	issuesInfo.Message = issuesInfoMarshal
	total = total + issuesInfo.Issue
	checkResultDetailsList = append(checkResultDetailsList, issuesInfo)

	gasUsageForMethods.Issue = issues
	gasUsageForMethods.GasLimit = gasLimit
	gasUsageForMethodsMarshal, err := json.Marshal(gasUsageForMethodsList)
	if err != nil {
		return err
	}
	gasUsageForMethods.Message = gasUsageForMethodsMarshal
	total = total + gasUsageForMethods.Issue
	checkResultDetailsList = append(checkResultDetailsList, gasUsageForMethods)

	gasUsageForDeployments.Issue = issues
	gasUsageForDeployments.GasLimit = gasLimit
	gasUsageForDeploymentsMarshal, err := json.Marshal(gasUsageForDeploymentsList)
	if err != nil {
		return err
	}
	gasUsageForDeployments.Message = gasUsageForDeploymentsMarshal
	total = total + gasUsageForDeployments.Issue
	checkResultDetailsList = append(checkResultDetailsList, gasUsageForDeployments)

	var result string
	if successFlag {
		result = consts.CheckSuccess.Result
	} else {
		result = consts.CheckFail.Result
	}
	checkResult := model.NewContractCheckResult(consts.EthGasCheckReport.Name, result, consts.EthGasCheckReport.Tool, checkResultDetailsList, total)
	checkResult.SolcVersion = solcVersion
	create, err := os.Create(path2.Join(a.path, consts.CheckResult))
	fmt.Println(checkResult)
	if err != nil {
		return err
	}
	marshal, err := json.Marshal(checkResult)
	if err != nil {
		return err
	}
	_, err = create.WriteString(strings.ReplaceAll(string(marshal), "\\u001b", ""))
	if err != nil {
		return err
	}
	create.Close()
	return nil
}

func (a *EthGasReporterAction) ExecuteCommand(commands []string, workdir string) (string, error) {
	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	logger.Debugf("execute eth-gas-reporter check command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))
	out, err := c.CombinedOutput()
	fmt.Println(string(out))
	a.output.WriteCommandLine(string(out))
	if err != nil {
		a.output.WriteLine(err.Error())
	}
	return string(out), err
}
