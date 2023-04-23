package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"github.com/jinzhu/copier"
	"log"
	"strings"
)

type MetaScanCheckAction struct {
	engineType     string
	scanToken      string
	projectName    string
	projectUrl     string
	organizationId string
	ctx            context.Context
	output         *output.Output
}

func NewMetaScanCheckAction(step model.Step, ctx context.Context, output *output.Output) *MetaScanCheckAction {
	return &MetaScanCheckAction{
		engineType:     step.With["engine_type"],
		scanToken:      step.With["scan_token"],
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
	data, err := GetProjectList(m.projectName, m.scanToken, m.organizationId)
	if err != nil {
		m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
		return nil, err
	}
	projectId := ""
	if len(data.Data.Items) > 0 {
		projectId = data.Data.Items[0].Id
	} else {
		project, err := CreateMetaScanProject(m.projectName, m.projectUrl, m.scanToken, m.organizationId)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		projectId = project.Data.Id
	}
	//2.start scan
	logger.Info("start scan ----------")
	startTaskRes, err := StartScanTask(projectId, m.engineType, m.scanToken, m.organizationId)
	if err != nil {
		m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
		return nil, err
	}
	logger.Info("start query task status")
	for {
		taskStatusRes, err := QueryTaskStatus(startTaskRes.Data.Id, m.scanToken, m.organizationId)
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
		logger.Info("query meta scan overview----------")
		engineTaskSummaryRes, err := GetEngineTaskSummary(startTaskRes.Data.EngineTasks[0].Id, m.scanToken, m.organizationId)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", "query task summary failed,save task summary failed"))
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		overview, err := json.Marshal(engineTaskSummaryRes.Data.ResultOverview.Impact)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		total := engineTaskSummaryRes.Data.ResultOverview.Impact.Medium + engineTaskSummaryRes.Data.ResultOverview.Impact.Low + engineTaskSummaryRes.Data.ResultOverview.Impact.Informational +
			engineTaskSummaryRes.Data.ResultOverview.Impact.High + engineTaskSummaryRes.Data.ResultOverview.Impact.Critical
		metaScanReport.Total = int64(total)
		metaScanReport.ResultOverview = string(overview)
		logger.Info("query meta scan result ----------------")
		checkResult, err := m.getTaskResult(startTaskRes.Data.EngineTasks[0].Id)
		if err != nil {
			m.output.WriteLine(fmt.Sprintf("[ERROR]: %s", err.Error()))
			return nil, err
		}
		metaScanReport.CheckResult = checkResult
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

func GetProjectList(title, token, organizationId string) (MetaProjectRes, error) {
	url := "https://app.metatrust.io/api/project"
	var result MetaProjectRes
	res, err := utils.NewHttp().NewRequest().SetQueryParams(map[string]string{
		"title": title,
	}).SetResult(&result).
		SetHeaders(map[string]string{
			"Authorization":  token,
			"X-MetaScan-Org": organizationId,
		}).Get(url)
	if err != nil {
		logger.Errorf("get meta scan projects failed: %s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		logger.Errorf("No permission to obtain meta scan projects:%s", res.Error())
		return result, errors.New("No permission to obtain meta scan projects")
	}
	if res.StatusCode() != 200 {
		logger.Errorf("get project list failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
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

func CreateMetaScanProject(title, repoUrl, token, organizationId string) (CreateProjectRes, error) {
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
	if err != nil {
		logger.Errorf("create meta scan project failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		logger.Errorf("No permission to obtain meta scan projects:%s", res.Error())
		return result, errors.New("No permission to create meta scan project")
	}
	if res.StatusCode() != 200 {
		logger.Errorf("get project list failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}

type CreateProjectRes struct {
	Message string          `json:"message"`
	Data    MetaScanProject `json:"data"`
	Code    int64           `json:"code"`
}

func StartScanTask(projectId, engineType, token, organizationId string) (StartTaskRes, error) {
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
	if err != nil {
		logger.Errorf("start scan failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		logger.Errorf("No permission to obtain meta scan projects:%s", res.Error())
		return result, errors.New("No permission to start scan")
	}
	if res.StatusCode() != 200 {
		logger.Errorf("get project list failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
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

func QueryTaskStatus(taskId, token, organizationId string) (TaskStatusRes, error) {
	url := "https://app.metatrust.io/api/scan/state"
	var result TaskStatusRes
	res, err := utils.NewHttp().NewRequest().SetQueryParam("taskId", taskId).SetHeaders(map[string]string{
		"Authorization":  token,
		"X-MetaScan-Org": organizationId,
	}).SetResult(&result).Get(url)
	if err != nil {
		logger.Errorf("http request query task status failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		logger.Errorf("No permission to query task status :%s", res.Error())
		return result, errors.New("No permission to query task status")
	}
	if res.StatusCode() != 200 {
		logger.Errorf("query task status failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
}

type TaskStatusRes struct {
	Data TaskStatus `json:"data"`
}

type TaskStatus struct {
	State string `json:"state"`
}

func GetEngineTaskSummary(engineTaskId, token, organizationId string) (SummaryDataRes, error) {
	url := "https://app.metatrust.io/api/scan/engineTask/{engineTaskId}"
	var result SummaryDataRes
	res, err := utils.NewHttp().NewRequest().SetPathParam("engineTaskId", engineTaskId).SetHeaders(map[string]string{
		"Authorization":  token,
		"X-MetaScan-Org": organizationId,
	}).SetResult(&result).Get(url)
	log.Println(res.StatusCode())
	if err != nil {
		logger.Errorf("http request query engine task summary failed:%s", err)
		return result, err
	}
	if res.StatusCode() == 401 {
		logger.Errorf("No permission to query engine task summary :%s", res.Error())
		return result, errors.New("No permission to query engine task summary")
	}
	if res.StatusCode() != 200 {
		logger.Errorf("query engine task summary failed:%s", res.Error())
		return result, errors.New(fmt.Sprintf("%v", res.Error()))
	}
	return result, nil
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

func (m *MetaScanCheckAction) getTaskResult(engineTaskId string) (string, error) {
	url := "https://app.metatrust.io/api/scan/history/engine/{engineTaskId}/result"
	var result TaskResultRes
	res, err := utils.NewHttp().NewRequest().SetPathParam("engineTaskId", engineTaskId).SetHeaders(map[string]string{
		"Authorization":  m.scanToken,
		"X-MetaScan-Org": m.organizationId,
	}).SetResult(&result).Get(url)
	if err != nil {
		logger.Errorf("http request query engine task result failed:%s", err)
		return "", err
	}
	if res.StatusCode() == 401 {
		logger.Errorf("No permission to query engine task result :%s", res.Error())
		return "", errors.New("No permission to query engine task result")
	}
	if res.StatusCode() != 200 {
		logger.Errorf("query engine task result failed:%s", res.Error())
		return "", errors.New(fmt.Sprintf("%v", res.Error()))
	}
	switch m.engineType {
	case "STATIC":
		resultReport, err := formatSAData(result)
		return resultReport, err
	default:
		return result.Data.Result, nil
	}
}

func formatSAData(result TaskResultRes) (string, error) {
	var resultData SecurityAnalyzerResponse
	if err := json.Unmarshal([]byte(result.Data.Result), &resultData); err != nil {
		log.Println("json unmarshal is failed")
		return "", err
	}
	var files []AffectedFile
	for _, analyzerResult := range resultData.Results {
		if len(analyzerResult.AffectedFiles) > 0 {
			files = append(files, analyzerResult.AffectedFiles...)
		}
	}
	groups := make(map[string][]AffectedFile)
	resultMap := make(map[string][]FormatSecurityAnalyzerResult)
	for _, p := range files {
		groups[p.Filepath] = append(groups[p.Filepath], p)
	}
	for _, analyzerResult := range resultData.Results {
		for _, file := range analyzerResult.AffectedFiles {
			_, ok := groups[file.Filepath]
			if ok {
				_, ok := resultMap[file.Filepath]
				if !ok {
					var newData []FormatSecurityAnalyzerResult
					resultMap[file.Filepath] = newData
				}
				resultMapData, _ := resultMap[file.Filepath]
				var data FormatSecurityAnalyzerResult
				var engine FormatMalwareWorkflowEngine
				copier.Copy(&engine, &analyzerResult.Mwe)
				data.Filepath = file.Filepath
				engine.ShowTitle = analyzerResult.ShowTitle
				engine.Hightlights = file.Hightlights
				engine.LineStart = file.LineStart
				engine.LineEnd = file.LineEnd
				data.Mwe = append(data.Mwe, engine)
				data.FileKey = resultData.FileMapping[file.Filepath]
				resultMapData = append(resultMapData, data)
				resultMap[file.Filepath] = resultMapData
			}
		}
	}
	jsonData, err := json.Marshal(&resultMap)
	if err != nil {
		logger.Errorf("json marshal is failed:%s", err)
	}
	return string(jsonData), nil
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

func GetFile() {
	url := "https://app.metatrust.io/api/scan/history/vulnerability-files/8f82c7ad-cacd-418a-bcd2-cbb4218c3f86/Functions.sol"
	res, err := utils.NewHttp().NewRequest().SetHeaders(map[string]string{
		"Authorization":  "Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJiYXdqN2JZQjdHX2MtVDJXNmFiQkIwMHZld2xoaHZLVVNfSXJUTDFBdUs4In0.eyJleHAiOjE2ODIwNzIxMjMsImlhdCI6MTY4MjA3MDMyMywianRpIjoiOTA4NjE5NzgtNjgyZC00MDJhLTkxYTQtZWNlNDZiZDJkZDdlIiwiaXNzIjoiaHR0cHM6Ly9hY2NvdW50Lm1ldGF0cnVzdC5pby9yZWFsbXMvbXQiLCJhdWQiOiJhY2NvdW50Iiwic3ViIjoiMjEzNjdlMGQtYWQ0NC00YTMwLWI4OWUtMDRmNDM2NWE4ZmM3IiwidHlwIjoiQmVhcmVyIiwiYXpwIjoid2ViYXBwIiwic2Vzc2lvbl9zdGF0ZSI6IjEyNjg4ZjkxLTFhNDQtNGIwZS1iMjQzLTdjNDg3OTIxZjE2NSIsImFjciI6IjEiLCJhbGxvd2VkLW9yaWdpbnMiOlsiaHR0cHM6Ly9hcHAubWV0YXRydXN0LmlvIiwiaHR0cHM6Ly9tZXRhdHJ1c3QuaW8iLCJodHRwczovL3d3dy5tZXRhdHJ1c3QuaW8iXSwicmVhbG1fYWNjZXNzIjp7InJvbGVzIjpbImRlZmF1bHQtcm9sZXMtbXQiLCJvZmZsaW5lX2FjY2VzcyIsInVtYV9hdXRob3JpemF0aW9uIl19LCJyZXNvdXJjZV9hY2Nlc3MiOnsiYWNjb3VudCI6eyJyb2xlcyI6WyJtYW5hZ2UtYWNjb3VudCIsIm1hbmFnZS1hY2NvdW50LWxpbmtzIiwiZGVsZXRlLWFjY291bnQiLCJ2aWV3LXByb2ZpbGUiXX19LCJzY29wZSI6ImVtYWlsIHByb2ZpbGUiLCJzaWQiOiIxMjY4OGY5MS0xYTQ0LTRiMGUtYjI0My03YzQ4NzkyMWYxNjUiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwicHJlZmVycmVkX3VzZXJuYW1lIjoidG9tQGhhbXN0ZXJuZXQuaW8iLCJlbWFpbCI6InRvbUBoYW1zdGVybmV0LmlvIiwidXNlcm5hbWUiOiJ0b21AaGFtc3Rlcm5ldC5pbyJ9.r_jaYmHjlHHDN2e93pHF9OKhUNpdZuZv5lUOrjlEWGtY0VsR2KIVu0SZVow0ygB6BatmKo10gdZliFGBl5mqbYjPhcvpmc8QRNXXJt2E80k9wc4gL1wtUWkds3wrBVDNpQ4PoxOvAIupPOKPLeA6R1OrnGsFgZBXy34ybc8gcTUGjNeuuWHTs6efdFhkFs7kX0LE1FnN6827LfL-Igi5XMVKcTpeJZhMTr-Mb4yGsZCtXZt_MSIlkvcbBE44jgNRB4eaCEGCbiagHjPe5ZFejZ8Q-Hf8gjkRxRx4x3uBxAHyjJgbrdhwilV4RALJT0w8AMrzPyJoG2JrtrSsGK2tDw",
		"X-MetaScan-Org": "1098616244203945984",
	}).Get(url)
	if err != nil {
		log.Println("获取失败")
		return
	}
	log.Println(res.StatusCode())
	log.Println(string(res.Body()))
}
