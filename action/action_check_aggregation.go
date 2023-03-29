package action

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"os"
	path2 "path"
	"strconv"
	"strings"
)

// CheckAggregationAction 合约聚合
type CheckAggregationAction struct {
	path   string
	ctx    context.Context
	output *output.Output
}

func NewCheckAggregationAction(step model.Step, ctx context.Context, output *output.Output) *CheckAggregationAction {
	return &CheckAggregationAction{
		path:   step.With["path"],
		ctx:    ctx,
		output: output,
	}
}

func (a *CheckAggregationAction) Pre() error {
	return nil
}

func (a *CheckAggregationAction) Hook() (*model.ActionResult, error) {
	// a.output.NewStep("check-aggregation")

	stack := a.ctx.Value(STACK).(map[string]interface{})
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
	destDir := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.CheckName, jobId)
	absPathList = utils.GetSameFileNameFiles(destDir, consts.CheckResult, absPathList)
	_, err = os.Stat(destDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	var checkResultList []model.ContractCheckResult[json.RawMessage]
	//var styleGuideValidationsReportList model.ContractCheckResult[[]model.ContractStyleGuideValidationsReportDetails]
	//var securityAnalysisReportList model.ContractCheckResult[[]model.ContractStyleGuideValidationsReportDetails]
	for _, path := range absPathList {
		file, err := os.ReadFile(path)
		if err != nil {
			return nil, errors.New("file open fail")
		}
		result := string(file)
		if strings.Contains(result, consts.ContractMethodsPropertiesReport.Name) {
			var methodsPropertiesReport model.ContractCheckResult[[]model.ContractMethodsPropertiesReportDetails]
			err := json.Unmarshal(file, &methodsPropertiesReport)
			if err != nil {
				continue
			}
			var methodsPropertiesReportRaw model.ContractCheckResult[json.RawMessage]
			methodsPropertiesReportRaw.Tool = methodsPropertiesReport.Tool
			methodsPropertiesReportRaw.Name = methodsPropertiesReport.Name
			methodsPropertiesReportRaw.Result = methodsPropertiesReport.Result
			var contractCheckResultDetailsList []model.ContractCheckResultDetails[json.RawMessage]
			for _, report := range methodsPropertiesReport.Context {
				var contractCheckResultDetails model.ContractCheckResultDetails[json.RawMessage]
				contractCheckResultDetails.Name = report.Name
				contractCheckResultDetails.Issue = report.Issue
				marshal, err := json.Marshal(report.Message)
				if err != nil {
					continue
				}
				contractCheckResultDetails.Message = []byte(strings.ReplaceAll(string(marshal), "\\u001b", ""))
				contractCheckResultDetailsList = append(contractCheckResultDetailsList, contractCheckResultDetails)
			}
			methodsPropertiesReportRaw.Context = contractCheckResultDetailsList
			checkResultList = append(checkResultList, methodsPropertiesReportRaw)
		}
		if strings.Contains(result, consts.ContractStyleGuideValidationsReport.Name) {
			var styleGuideValidationsReport model.ContractCheckResult[[]model.ContractStyleGuideValidationsReportDetails]
			err := json.Unmarshal(file, &styleGuideValidationsReport)
			if err != nil {
				continue
			}
			var styleGuideValidationsReportRaw model.ContractCheckResult[json.RawMessage]
			styleGuideValidationsReportRaw.Tool = styleGuideValidationsReport.Tool
			styleGuideValidationsReportRaw.Name = styleGuideValidationsReport.Name
			styleGuideValidationsReportRaw.Result = styleGuideValidationsReport.Result
			var contractCheckResultDetailsList []model.ContractCheckResultDetails[json.RawMessage]
			for _, report := range styleGuideValidationsReport.Context {
				var contractCheckResultDetails model.ContractCheckResultDetails[json.RawMessage]
				contractCheckResultDetails.Name = report.Name
				contractCheckResultDetails.Issue = report.Issue
				marshal, err := json.Marshal(report.Message)
				if err != nil {
					continue
				}
				contractCheckResultDetails.Message = marshal
				contractCheckResultDetailsList = append(contractCheckResultDetailsList, contractCheckResultDetails)
			}
			styleGuideValidationsReportRaw.Context = contractCheckResultDetailsList
			checkResultList = append(checkResultList, styleGuideValidationsReportRaw)
		}
		if strings.Contains(result, consts.ContractSecurityAnalysisReport.Name) || strings.Contains(result, consts.FormalSpecificationAndVerificationReport.Name) {
			var securityAnalysisReport model.ContractCheckResult[[]model.ContractStyleGuideValidationsReportDetails]
			err := json.Unmarshal(file, &securityAnalysisReport)
			if err != nil {
				continue
			}
			var securityAnalysisReportRaw model.ContractCheckResult[json.RawMessage]
			securityAnalysisReportRaw.Tool = securityAnalysisReport.Tool
			securityAnalysisReportRaw.Name = securityAnalysisReport.Name
			securityAnalysisReportRaw.Result = securityAnalysisReport.Result
			var contractCheckResultDetailsList []model.ContractCheckResultDetails[json.RawMessage]
			for _, report := range securityAnalysisReport.Context {
				var contractCheckResultDetails model.ContractCheckResultDetails[json.RawMessage]
				contractCheckResultDetails.Name = report.Name
				contractCheckResultDetails.Issue = report.Issue
				marshal, err := json.Marshal(report.Message)
				if err != nil {
					continue
				}
				contractCheckResultDetails.Message = marshal
				contractCheckResultDetailsList = append(contractCheckResultDetailsList, contractCheckResultDetails)
			}
			securityAnalysisReportRaw.Context = contractCheckResultDetailsList
			checkResultList = append(checkResultList, securityAnalysisReportRaw)
		}
		if strings.Contains(result, consts.FrontEndCheckReport.Name) {
			var eslintCheckReportReport model.ContractCheckResult[[]model.EslintCheckReportDetails]
			err := json.Unmarshal(file, &eslintCheckReportReport)
			if err != nil {
				continue
			}
			var eslintCheckReportReportRaw model.ContractCheckResult[json.RawMessage]
			eslintCheckReportReportRaw.Tool = eslintCheckReportReport.Tool
			eslintCheckReportReportRaw.Name = eslintCheckReportReport.Name
			eslintCheckReportReportRaw.Result = eslintCheckReportReport.Result
			var eslintCheckResultDetailsList []model.ContractCheckResultDetails[json.RawMessage]
			for _, report := range eslintCheckReportReport.Context {
				var contractCheckResultDetails model.ContractCheckResultDetails[json.RawMessage]
				contractCheckResultDetails.Name = report.Name
				contractCheckResultDetails.Issue = report.Issue
				marshal, err := json.Marshal(report.Message)
				if err != nil {
					continue
				}
				contractCheckResultDetails.Message = marshal
				eslintCheckResultDetailsList = append(eslintCheckResultDetailsList, contractCheckResultDetails)
			}
			eslintCheckReportReportRaw.Context = eslintCheckResultDetailsList
			checkResultList = append(checkResultList, eslintCheckReportReportRaw)
		}
		if strings.Contains(result, consts.EthGasCheckReport.Name) {
			var ethGasReportReportRaw model.ContractCheckResult[json.RawMessage]
			err := json.Unmarshal(file, &ethGasReportReportRaw)
			if err != nil {
				continue
			}
			checkResultList = append(checkResultList, ethGasReportReportRaw)
		}
	}
	a.path = path2.Join(destDir, consts.CheckAggregationResult)
	create, err := os.Create(a.path)
	if err != nil {
		return nil, err
	}
	marshal, err := json.Marshal(checkResultList)
	if err != nil {
		return nil, err
	}
	_, err = create.WriteString(string(marshal))
	if err != nil {
		return nil, err
	}
	create.Close()

	id, err := strconv.Atoi(jobId)
	if err != nil {
		return nil, err
	}
	actionResult := model.ActionResult{
		Artifactorys: nil,
		Reports: []model.Report{
			{
				Id:   id,
				Url:  a.path,
				Type: 2,
			},
		},
	}
	return &actionResult, err
}

func (a *CheckAggregationAction) Post() error {
	return nil
}
