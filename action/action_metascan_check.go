package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"github.com/jinzhu/copier"
	"github.com/tidwall/gjson"
	"log"
	"os"
	"strings"
)

type MetaScanCheckAction struct {
	engineType     string
	scanToken      string
	projectName    string
	projectUrl     string
	organizationId string
	tool           string
	ctx            context.Context
	output         *output.Output
}

func NewMetaScanCheckAction(step model.Step, ctx context.Context, output *output.Output) *MetaScanCheckAction {
	return &MetaScanCheckAction{
		engineType:     step.With["engine_type"],
		scanToken:      step.With["scan_token"],
		tool:           step.With["tool"],
		projectName:    step.With["project_name"],
		projectUrl:     step.With["project_url"],
		organizationId: step.With["organization_id"],
		ctx:            ctx,
		output:         output,
	}
}

func (m *MetaScanCheckAction) Pre() error {
	stack := m.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	logger.Debugf("engine type is : %s", m.engineType)
	m.scanToken = utils.ReplaceWithParam(m.scanToken, params)
	logger.Debugf("token is : %s", m.scanToken)
	m.projectName = utils.ReplaceWithParam(m.projectName, params)
	logger.Debugf("project name is : %s", m.projectName)
	m.projectUrl = utils.ReplaceWithParam(m.projectUrl, params)
	logger.Debugf("project url is : %s", m.projectUrl)
	return nil
}

func (m *MetaScanCheckAction) Hook() (*model.ActionResult, error) {
	//1.query project and create project
	logger.Info("query meta scan project list ------")
	data, err := m.metaScanGetProjects()
	if err != nil {
		m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
		return nil, err
	}
	projectId := ""
	if len(data.Data.Items) > 0 {
		projectId = data.Data.Items[0].Id
	} else {
		project, err := m.metaScanCreateProject()
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		projectId = project.Data.Id
	}
	//2.start scan
	logger.Info("start scan ----------")
	startTaskRes, err := m.metaScanStartScanTask(projectId)
	if err != nil {
		m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
		return nil, err
	}
	logger.Info("start query task status")
	for {
		taskStatusRes, err := m.metaScanQueryTaskStatus(startTaskRes.Data.Id)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		logger.Infof("task status is %s", taskStatusRes.Data.State)
		if taskStatusRes.Data.State == "SUCCESS" {
			logger.Info("task status is success")
			break
		} else if taskStatusRes.Data.State == "FAILURE" || taskStatusRes.Data.State == "SYS_ABORT" || taskStatusRes.Data.State == "ABORT" {
			logger.Info("task status is failed")
			break
		} else {
			continue
		}
	}
	logger.Info("task status is success")

	actionResult := &model.ActionResult{}
	metaScanReport := model.MetaScanReport{}
	if len(startTaskRes.Data.EngineTasks) > 0 {
		logger.Info("query meta scan result ----------------")
		checkResult, overviewData, err := m.metaScanGetTaskResult(startTaskRes.Data.EngineTasks[0].Id)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		metaScanReport.CheckResult = checkResult
		metaScanReport.Tool = m.tool
		logger.Info("query meta scan overview----------")
		if m.engineType != "STATIC" {
			engineTaskSummaryRes, err := m.metaScanGetEngineTaskSummary(startTaskRes.Data.EngineTasks[0].Id)
			if err != nil {
				m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", "query task summary failed,save task summary failed"))
				m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
				return nil, err
			}
			overviewData = engineTaskSummaryRes.Data.ResultOverview.Impact
		}
		total := overviewData.High + overviewData.Low + overviewData.Medium + overviewData.Informational + overviewData.Critical
		overview, err := json.Marshal(overviewData)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		metaScanReport.Total = int64(total)
		metaScanReport.ResultOverview = string(overview)
		actionResult.MetaScanData = append(actionResult.MetaScanData, metaScanReport)
		return actionResult, nil
	} else {
		logger.Info("engine task is empty")
		m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", "start engine task list is empty"))
		return nil, errors.New("start engine task list is empty")
	}
}

func (m *MetaScanCheckAction) Post() error {
	return nil
}

func (m *MetaScanCheckAction) metaScanGetProjects() (MetaProjectRes, error) {
	res, result, err := getProjectList(m.projectName, m.scanToken, m.organizationId)
	if err != nil {
		logger.Errorf("get meta scan projects failed: %s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		token := metaScanHttpRequestToken()
		if token != "" {
			m.scanToken = token
			stack := m.ctx.Value(STACK).(map[string]interface{})
			params := stack["parameter"].(map[string]string)
			params["scanToken"] = token
		} else {
			return result, errors.New("Failed to retrieve token again")
		}
		res, result, err = getProjectList(m.projectName, m.scanToken, m.organizationId)
		if err != nil {
			logger.Errorf("get meta scan projects failed again: %s", err)
			return result, err
		}
		if res.StatusCode() == 401 {
			return result, errors.New("No permission to obtain meta scan projects")
		}
	}
	if res.StatusCode() != 200 {
		logger.Errorf("get project list failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}

func getProjectList(title, token, organizationId string) (*resty.Response, MetaProjectRes, error) {
	url := "https://app.metatrust.io/api/project"
	var result MetaProjectRes
	res, err := utils.NewHttp().NewRequest().SetQueryParams(map[string]string{
		"title": title,
	}).SetResult(&result).
		SetHeaders(map[string]string{
			"Authorization":  token,
			"X-MetaScan-Org": organizationId,
		}).Get(url)
	return res, result, err
}

type MetaScanProject struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

type MetaProjectRes struct {
	Data MetaProjectsData `json:"data"`
}

type MetaProjectsData struct {
	Total      int               `json:"total"`
	TotalPages int               `json:"totalPages"`
	Items      []MetaScanProject `json:"items"`
}

func (m *MetaScanCheckAction) metaScanCreateProject() (CreateProjectRes, error) {
	res, result, err := createMetaScanProject(m.projectName, m.projectUrl, m.scanToken, m.organizationId)
	if err != nil {
		logger.Errorf("create meta scan project failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		token := metaScanHttpRequestToken()
		if token != "" {
			m.scanToken = token
			stack := m.ctx.Value(STACK).(map[string]interface{})
			params := stack["parameter"].(map[string]string)
			params["scanToken"] = token
		} else {
			return result, errors.New("Failed to retrieve token again")
		}
		res, result, err = createMetaScanProject(m.projectName, m.projectUrl, m.scanToken, m.organizationId)
		if err != nil {
			logger.Errorf("Again create meta scan project failed:%s", err)
			return result, err
		}
		if res.StatusCode() == 401 {
			logger.Errorf("Again no permission to obtain meta scan projects:%s", res.Error())
			return result, errors.New("Again no permission to create meta scan project")
		}
	}
	if res.StatusCode() != 200 {
		logger.Errorf("get project list failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}

func createMetaScanProject(title, repoUrl, token, organizationId string) (*resty.Response, CreateProjectRes, error) {
	url := "https://app.metatrust.io/api/project"
	githubUrl := handleRepoUrl(repoUrl)
	createData := struct {
		Title           string `json:"title"`
		RepoUrl         string `json:"repoUrl"`
		IntegrationType string `json:"integrationType"`
	}{
		Title:           title,
		RepoUrl:         githubUrl,
		IntegrationType: "github",
	}
	var result CreateProjectRes
	res, err := utils.NewHttp().NewRequest().SetBody(&createData).
		SetHeaders(map[string]string{
			"Authorization":  token,
			"X-MetaScan-Org": organizationId,
			"Content-Type":   "application/json",
		}).SetResult(&result).Post(url)
	return res, result, err
}

type CreateProjectRes struct {
	Message string          `json:"message"`
	Data    MetaScanProject `json:"data"`
	Code    int64           `json:"code"`
}

func (m *MetaScanCheckAction) metaScanStartScanTask(projectId string) (StartTaskRes, error) {
	res, result, err := startScanTask(projectId, m.engineType, m.scanToken, m.organizationId)
	if err != nil {
		logger.Errorf("start scan failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		token := metaScanHttpRequestToken()
		if token != "" {
			m.scanToken = token
			stack := m.ctx.Value(STACK).(map[string]interface{})
			params := stack["parameter"].(map[string]string)
			params["scanToken"] = token
		} else {
			return result, errors.New("Failed to retrieve token again")
		}
		res, result, err = startScanTask(projectId, m.engineType, m.scanToken, m.organizationId)
		if err != nil {
			logger.Errorf("Again start scan failed:%s", err)
			return result, err
		}
		if res.StatusCode() == 401 {
			logger.Errorf("Again no permission to obtain meta scan projects:%s", res.Error())
			return result, errors.New("Again no permission to start scan")
		}
	}
	if res.StatusCode() != 200 {
		logger.Errorf("get project list failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}

func startScanTask(projectId, engineType, token, organizationId string) (*resty.Response, StartTaskRes, error) {
	url := "https://app.metatrust.io/api/scan/task"
	var types []string
	types = append(types, engineType)
	repoData := Repo{
		Branch:     "",
		CommitHash: "",
	}
	scanData := TaskScan{
		SubPath:      "",
		Mode:         "",
		IgnoredPaths: "node_modules,test,tests,mock",
	}
	envData := TaskEnv{
		Node:           "",
		Solc:           "",
		PackageManage:  "",
		CompileCommand: "",
		Variables:      "",
	}
	bodyData := struct {
		EngineTypes []string `json:"engine_types"`
		Repo        Repo     `json:"repo"`
		Scan        TaskScan `json:"scan"`
		Env         TaskEnv  `json:"env"`
	}{
		EngineTypes: types,
		Repo:        repoData,
		Scan:        scanData,
		Env:         envData,
	}
	var result StartTaskRes
	res, err := utils.NewHttp().NewRequest().SetQueryParams(map[string]string{
		"action":    "start-scan",
		"projectId": projectId,
	}).SetBody(&bodyData).SetHeaders(map[string]string{
		"Authorization":  token,
		"X-MetaScan-Org": organizationId,
		"Content-Type":   "application/json",
	}).SetResult(&result).Post(url)
	return res, result, err
}

type Repo struct {
	Branch     string `json:"branch"`
	CommitHash string `json:"commit_hash"`
}
type StartTaskRes struct {
	Data TaskData `json:"data"`
}

type TaskData struct {
	Id          string       `json:"id"`
	TaskState   string       `json:"taskState"`
	EngineTasks []BaseEntity `json:"engineTasks"`
}

type BaseEntity struct {
	Id string `json:"id"`
}

type TaskScan struct {
	SubPath      string `json:"sub_path"`
	Mode         string `json:"mode"`
	IgnoredPaths string `json:"ignored_paths"`
}

type TaskEnv struct {
	Node           string `json:"node"`
	Solc           string `json:"solc"`
	PackageManage  string `json:"package_manage"`
	CompileCommand string `json:"compile_command"`
	Variables      string `json:"variables"`
}

func (m *MetaScanCheckAction) metaScanQueryTaskStatus(taskId string) (TaskStatusRes, error) {
	res, result, err := queryTaskStatus(taskId, m.scanToken, m.organizationId)
	if err != nil {
		logger.Errorf("http request query task status failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		token := metaScanHttpRequestToken()
		if token != "" {
			m.scanToken = token
			stack := m.ctx.Value(STACK).(map[string]interface{})
			params := stack["parameter"].(map[string]string)
			params["scanToken"] = token
		} else {
			return result, errors.New("Failed to retrieve token again")
		}
		res, result, err = queryTaskStatus(taskId, m.scanToken, m.organizationId)
		if err != nil {
			logger.Errorf("Again http request query task status failed:%s", err)
			return result, err
		}
		if res.StatusCode() == 401 {
			logger.Errorf("Again no permission to query task status :%s", res.Error())
			return result, errors.New("Again no permission to query task status")
		}
	}
	if res.StatusCode() != 200 {
		logger.Errorf("query task status failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}
func queryTaskStatus(taskId, token, organizationId string) (*resty.Response, TaskStatusRes, error) {
	url := "https://app.metatrust.io/api/scan/state"
	var result TaskStatusRes
	res, err := utils.NewHttp().NewRequest().SetQueryParam("taskId", taskId).SetHeaders(map[string]string{
		"Authorization":  token,
		"X-MetaScan-Org": organizationId,
	}).SetResult(&result).Get(url)
	return res, result, err
}

type TaskStatusRes struct {
	Data TaskStatus `json:"data"`
}

type TaskStatus struct {
	State string `json:"state"`
}

func (m *MetaScanCheckAction) metaScanGetEngineTaskSummary(engineTaskId string) (SummaryDataRes, error) {
	res, result, err := getEngineTaskSummary(engineTaskId, m.scanToken, m.organizationId)
	if err != nil {
		logger.Errorf("http request query engine task summary failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		token := metaScanHttpRequestToken()
		if token != "" {
			m.scanToken = token
			stack := m.ctx.Value(STACK).(map[string]interface{})
			params := stack["parameter"].(map[string]string)
			params["scanToken"] = token
		} else {
			return result, errors.New("Failed to retrieve token again")
		}
		res, result, err = getEngineTaskSummary(engineTaskId, m.scanToken, m.organizationId)
		if err != nil {
			logger.Errorf("Again http request query engine task summary failed:%s", err)
			return result, err
		}
		if res.StatusCode() == 401 {
			logger.Errorf("Again no permission to query engine task summary :%s", res.Error())
			return result, errors.New("Again no permission to query engine task summary")
		}
	}
	if res.StatusCode() != 200 {
		logger.Errorf("query engine task summary failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}

func getEngineTaskSummary(engineTaskId, token, organizationId string) (*resty.Response, SummaryDataRes, error) {
	url := "https://app.metatrust.io/api/scan/engineTask/{engineTaskId}"
	var result SummaryDataRes
	res, err := utils.NewHttp().NewRequest().SetPathParam("engineTaskId", engineTaskId).SetHeaders(map[string]string{
		"Authorization":  token,
		"X-MetaScan-Org": organizationId,
	}).SetResult(&result).Get(url)
	return res, result, err
}

type SummaryDataRes struct {
	Data SummaryData `json:"data"`
}

type SummaryData struct {
	ResultOverview ResultOverview `json:"resultOverview"`
}

type ResultOverview struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Impact  Impact `json:"impact"`
}

type Impact struct {
	Critical      int `json:"CRITICAL"`
	Low           int `json:"LOW"`
	High          int `json:"HIGH"`
	Medium        int `json:"MEDIUM"`
	Informational int `json:"INFORMATIONAL"`
}

//string:check result string:overview
func (m *MetaScanCheckAction) metaScanGetTaskResult(engineTaskId string) (string, Impact, error) {
	var impact Impact
	res, result, err := getTaskResult(engineTaskId, m.scanToken, m.organizationId)
	if err != nil {
		logger.Errorf("http request query engine task result failed:%s", err)
		return "", impact, err
	}
	if res.StatusCode() == 401 {
		token := metaScanHttpRequestToken()
		if token != "" {
			m.scanToken = token
			stack := m.ctx.Value(STACK).(map[string]interface{})
			params := stack["parameter"].(map[string]string)
			params["scanToken"] = token
		} else {
			return "", impact, errors.New("Failed to retrieve token again")
		}
		res, result, err = getTaskResult(engineTaskId, m.scanToken, m.organizationId)
		if err != nil {
			logger.Errorf("Again http request query engine task result failed:%s", err)
			return "", impact, err
		}
		if res.StatusCode() == 401 {
			logger.Errorf("Again no permission to query engine task result :%s", res.Error())
			return "", impact, errors.New("Again no permission to query engine task result")
		}
	}
	if res.StatusCode() != 200 {
		logger.Errorf("query engine task result failed:%s", res.Error())
		return "", impact, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	switch m.engineType {
	case "STATIC":
		resultReport, impact, err := formatSAData(result)
		return resultReport, impact, err
	case "PROVER":
		resultReport, err := formatSPData(result)
		return resultReport, impact, err
	case "SCA":
		return result.Data.Result, impact, nil
	case "LINT":
		resultReport, err := formatCQData(result)
		return resultReport, impact, err
	default:
		return result.Data.Result, impact, nil
	}
}

func getTaskResult(engineTaskId, scanToken, organizationId string) (*resty.Response, TaskResultRes, error) {
	url := "https://app.metatrust.io/api/scan/history/engine/{engineTaskId}/result"
	var result TaskResultRes
	res, err := utils.NewHttp().NewRequest().SetPathParam("engineTaskId", engineTaskId).SetHeaders(map[string]string{
		"Authorization":  scanToken,
		"X-MetaScan-Org": organizationId,
	}).SetResult(&result).Get(url)
	return res, result, err
}

func formatSAData(result TaskResultRes) (string, Impact, error) {
	var impact Impact
	var resultData SecurityAnalyzerResponse
	if err := json.Unmarshal([]byte(result.Data.Result), &resultData); err != nil {
		log.Println("json unmarshal is failed")
		return "", impact, err
	}
	var files []AffectedFile
	for _, analyzerResult := range resultData.Results {
		if len(analyzerResult.AffectedFiles) > 0 {
			files = append(files, analyzerResult.AffectedFiles...)
		}
	}
	groups := make(map[string][]AffectedFile)
	resultMap := make(map[string]FormatSecurityAnalyzerResult)
	for _, p := range files {
		groups[p.Filepath] = append(groups[p.Filepath], p)
	}
	var critical int
	var low int
	var high int
	var medium int
	var informational int
	for _, analyzerResult := range resultData.Results {
		for _, file := range analyzerResult.AffectedFiles {
			_, ok := groups[file.Filepath]
			if ok {
				_, ok := resultMap[file.Filepath]
				if !ok {
					var newData FormatSecurityAnalyzerResult
					newData.FileKey = resultData.FileMapping[file.Filepath]
					newData.Filepath = file.Filepath
					resultMap[file.Filepath] = newData
				}
				resultMapData, _ := resultMap[file.Filepath]
				var engine FormatMalwareWorkflowEngine
				copier.Copy(&engine, &analyzerResult.Mwe)
				engine.ShowTitle = analyzerResult.ShowTitle
				engine.Hightlights = file.Hightlights
				engine.LineStart = file.LineStart
				engine.LineEnd = file.LineEnd
				resultMapData.Mwe = append(resultMapData.Mwe, engine)
				resultMap[file.Filepath] = resultMapData
				if engine.Severity == "Critical" {
					critical = critical + 1
				} else if engine.Severity == "High" {
					high = high + 1
				} else if engine.Severity == "Medium" {
					medium = medium + 1
				} else if engine.Severity == "LOW" {
					low = low + 1
				} else {
					informational = informational + 1
				}
			}
		}
	}
	impact.Critical = critical
	impact.Informational = informational
	impact.High = high
	impact.Low = low
	impact.Medium = medium
	jsonData, err := json.Marshal(&resultMap)
	if err != nil {
		logger.Errorf("json marshal is failed:%s", err)
	}
	return string(jsonData), impact, nil
}

type TaskResultRes struct {
	Data TaskResult `json:"data"`
}

type TaskResult struct {
	Title  string `json:"title"`
	Result string `json:"result"`
}

func handleRepoUrl(repoUrl string) string {
	s := strings.TrimPrefix(repoUrl, "https://github.com/")
	s = strings.TrimSuffix(s, ".git")
	return s
}

type SecurityAnalyzerResponse struct {
	Success     bool                     `json:"success"`
	Error       string                   `json:"error"`
	Results     []SecurityAnalyzerResult `json:"results"`
	FileMapping map[string]string        `json:"file_mapping"`
}

type SecurityAnalyzerResult struct {
	Mwe           MalwareWorkflowEngine `json:"mwe"`
	ShowTitle     string                `json:"show_title"`
	AffectedFiles []AffectedFile        `json:"affected_files"`
}

type AffectedFile struct {
	Filepath         string `json:"filepath"`
	FilepathRelative string `json:"filepath_relative"`
	Hightlights      []int  `json:"hightlights"`
	LineStart        int    `json:"line_start"`
	LineEnd          int    `json:"line_end"`
}

type MalwareWorkflowEngine struct {
	Id             string `json:"id"`
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Title          string `json:"title"`
	Confidence     string `json:"confidence"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

type FormatMalwareWorkflowEngine struct {
	MalwareWorkflowEngine
	ShowTitle   string `json:"showTitle"`
	Hightlights []int  `json:"hightlights"`
	LineStart   int    `json:"lineStart"`
	LineEnd     int    `json:"lineEnd"`
}

type FormatSecurityAnalyzerResult struct {
	Filepath string                        `json:"filepath"`
	Mwe      []FormatMalwareWorkflowEngine `json:"mwe"`
	FileKey  string                        `json:"fileKey"`
}

func formatCQData(results TaskResultRes) (string, error) {
	result := results.Data.Result
	var cqJson CQJson
	FileMapping := make(map[string]string)
	cqJson.Issues = make(map[string]Issue)
	cqJson.Version = gjson.Get(result, "version").String()
	cqJson.Success = gjson.Get(result, "success").Bool()
	fileMap := gjson.Get(result, "file_mapping").Map()
	for k, v := range fileMap {
		FileMapping[k] = v.String()
	}
	cqJson.Message = gjson.Get(result, "message").String()
	for _, v := range gjson.Get(result, "results").Array() {
		code := v.Get("code").String()
		category := v.Get("category").String()
		severity := v.Get("severity").String()
		title := v.Get("title").String()
		description := v.Get("description").String()
		for _, af := range v.Get("affectedFiles").Array() {
			filePath := af.Get("filePath").String()
			line := af.Get("range").Get("start").Get("line").Int()
			column := af.Get("range").Get("start").Get("column").Int()
			highlightsArray := af.Get("highlights").Array()
			var highlights []int64
			for _, hl := range highlightsArray {
				highlights = append(highlights, hl.Int())
			}
			detail := Detail{
				Code:        code,
				Category:    category,
				Severity:    severity,
				Title:       title,
				Description: description,
				AffectedFiles: CQAffectedFile{
					Line:       line,
					Column:     column,
					Highlights: highlights,
				},
			}
			issues := cqJson.Issues[filePath]
			issues.FilePath = filePath
			issues.FileAddress = FileMapping[filePath]
			issues.Details = append(issues.Details, detail)
			cqJson.Issues[filePath] = issues
		}
	}
	jsonData, err := json.Marshal(&cqJson)
	if err != nil {
		logger.Errorf("json marshal is failed:%s", err)
	}
	return string(jsonData), nil
}

type CQJson struct {
	Version string
	Success bool
	Message string
	Issues  map[string]Issue
}

type Issue struct {
	FilePath    string
	FileAddress string
	// 该文件具有的问题
	Details []Detail
}

// Detail 具体出现的问题
type Detail struct {
	Code          string
	Category      string
	Severity      string
	Title         string
	Description   string
	AffectedFiles CQAffectedFile
}

type CQAffectedFile struct {
	Line       int64
	Column     int64
	Highlights []int64
}

func formatSPData(results TaskResultRes) (string, error) {
	result := results.Data.Result
	var resultData SecurityProverResponse
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		return "", err
	}
	var spJson SPJson
	spJson.Version = resultData.Version
	spJson.Success = resultData.Success
	spJson.Message = resultData.Message
	FileMapping := make(map[string]string)
	spJson.Issues = make(map[string]SPIssue)
	fileMap := resultData.FileMapping
	for k, v := range fileMap {
		FileMapping[k] = v
	}
	for _, r := range resultData.Results {
		for _, af := range r.AffectedFiles {
			spAf := SPAffectedFiles{
				Text:       af.Text,
				Range:      af.Range,
				Highlights: af.Highlights,
			}
			detail := SPDetail{
				ID:            r.ID,
				Code:          r.Code,
				Severity:      r.Severity,
				Title:         r.Title,
				Description:   r.Description,
				Function:      r.Function,
				AffectedFiles: spAf,
				Poc:           r.Poc,
			}
			filePath := af.FilePath
			issues := spJson.Issues[filePath]
			issues.FilePath = filePath
			issues.FileAddress = FileMapping[filePath]
			issues.Details = append(issues.Details, detail)
			spJson.Issues[filePath] = issues
		}
	}
	jsonData, err := json.Marshal(&spJson)
	if err != nil {
		logger.Errorf("json marshal is failed:%s", err)
	}
	return string(jsonData), nil
}

type SPJson struct {
	Version string
	Success bool
	Message string
	Issues  map[string]SPIssue
}

type SPIssue struct {
	FilePath    string
	FileAddress string
	// 该文件具有的问题
	Details []SPDetail
}

type SPDetail struct {
	ID            string
	Code          string
	Severity      string
	Title         string
	Description   string
	Function      string          `json:"function"`
	AffectedFiles SPAffectedFiles `json:"affectedFiles"`
	Poc           []Poc           `json:"poc"`
}

type SPAffectedFiles struct {
	Text       string       `json:"text"`
	Range      Range        `json:"range"`
	Highlights []Highlights `json:"highlights"`
}

type SecurityProverResponse struct {
	Results     []SecurityProverResults `json:"securityProverResults"`
	FileMapping map[string]string       `json:"file_mapping"`
	Success     bool                    `json:"success"`
	Error       string                  `json:"error"`
	Message     string                  `json:"message"`
	Version     string                  `json:"version"`
}

type SecurityProverResults struct {
	ID            string          `json:"id"`
	Code          string          `json:"code"`
	Severity      string          `json:"severity"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Function      string          `json:"function"`
	AffectedFiles []AffectedFiles `json:"affectedFiles"`
	Poc           []Poc           `json:"poc"`
}

type Poc struct {
	FunctionName string        `json:"functionName"`
	Parameters   []interface{} `json:"parameters"`
	Location     string        `json:"location"`
	CallDepth    int           `json:"callDepth"`
}

type AffectedFiles struct {
	FilePath   string       `json:"filePath"`
	Text       string       `json:"text"`
	Range      Range        `json:"range"`
	Highlights []Highlights `json:"highlights"`
}

type Range struct {
	Start Start `json:"start"`
	End   End   `json:"end"`
}

type Highlights struct {
	Start Start `json:"start"`
	End   End   `json:"end"`
}

type Start struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}
type End struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func metaScanHttpRequestToken() string {
	url := "https://account.metatrust.io/realms/mt/protocol/openid-connect/token"
	token := struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshExpiresIn int64  `json:"refresh_expires_in"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		NotBeforePolicy  int    `json:"not-before-policy"`
		SessionState     string `json:"session_state"`
		Scope            string `json:"scope"`
	}{}
	res, err := utils.NewHttp().NewRequest().SetFormData(map[string]string{
		"grant_type": "password",
		"username":   os.Getenv("METASCAN_USERNAME"),
		"password":   os.Getenv("METASCAN_PASSWORD"),
		"client_id":  "webapp",
	}).SetResult(&token).SetHeader("Content-Type", "application/x-www-form-urlencoded").Post(url)
	if res.StatusCode() != 200 {
		return ""
	}
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)
	//return token.AccessToken
}
