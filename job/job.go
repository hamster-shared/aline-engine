package job

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/grpc/api"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"github.com/hamster-shared/aline-engine/utils/platform"
	"github.com/jinzhu/copier"
	"gopkg.in/yaml.v3"
)

// SaveJob 保存 Job yaml 文件
func SaveJob(name string, yaml string) error {
	return saveStringToFile(getJobFilePath(name), yaml)
}

func SaveJobParams(name string, params map[string]string) error {
	job, err := GetJobObject(name)

	if err != nil {
		return err
	}
	job.Parameter = params
	content, err := yaml.Marshal(job)
	if err != nil {
		return err
	}
	return SaveJob(job.Name, string(content))
}

func SaveJobUserId(name string, userId string) error {
	job, err := GetJobObject(name)
	job.UserId = userId
	if err != nil {
		return err
	}
	content, err := yaml.Marshal(job)
	if err != nil {
		return err
	}
	return SaveJob(job.Name, string(content))
}

// GetJob get job
func GetJob(name string) (string, error) {
	return readStringFromFile(getJobFilePath(name))
}

// UpdateJob update job yaml file
func UpdateJob(oldName string, newName string, yaml string) error {
	err := renameFile(getJobFilePath(oldName), getJobFilePath(newName))
	if err != nil {
		return err
	}
	return SaveJob(newName, yaml)
}

// DeleteJob delete job yaml file
func DeleteJob(name string) error {
	return deleteFile(getJobFilePath(name))
}

// SaveJobDetail  save job detail
func SaveJobDetail(name string, job *model.JobDetail) error {
	job.TriggerMode = consts.TRIGGER_MODE
	data, err := yaml.Marshal(job)
	if err != nil {
		logger.Errorf("serializes yaml failed: %s", err)
		return err
	}
	saveStringToFile(GetJobDetailFilePath(name, job.Id), string(data))
	return nil
}

func SaveStringJobDetail(name string, id int, content string) error {
	return saveStringToFile(GetJobDetailFilePath(name, id), content)
}

func ReadStringJobDetail(name string, id int) (string, error) {
	return readStringFromFile(GetJobDetailFilePath(name, id))
}

// UpdateJobDetail update job detail yaml file
func UpdateJobDetail(name string, job *model.JobDetail) error {
	return SaveJobDetail(name, job)
}

// GetJobDetail get job detail
func GetJobDetail(name string, id int) (*model.JobDetail, error) {
	var jobDetail model.JobDetail
	jobDetailString, err := readStringFromFile(GetJobDetailFilePath(name, id))
	if err != nil {
		return nil, err
	}

	//deserialization job detail yml file
	err = yaml.Unmarshal([]byte(jobDetailString), &jobDetail)
	if err != nil {
		logger.Errorf("get job,deserialization job detail file failed: %s", err.Error())
		return nil, err
	}

	runningStage := -1
	for index, stage := range jobDetail.Stages {
		if stage.Status == model.STATUS_RUNNING {
			runningStage = index
		}
	}

	if runningStage >= 0 && runningStage < len(jobDetail.Stages) {
		jobDetail.Stages[runningStage].Duration = time.Since(jobDetail.Stages[runningStage].StartTime).Milliseconds()
	}
	return &jobDetail, nil
}

// JobList  job list
func JobList(keyword string, page, pageSize int) (*model.JobPage, error) {
	var jobPage model.JobPage
	var jobs []model.JobVo
	//jobs folder path
	jobsDir := getJobFilePath("")
	if !isFileExist(jobsDir) {
		return nil, fmt.Errorf("jobs folder not exist: %s", jobsDir)
	}

	// 遍历 jobs 文件夹
	files, err := os.ReadDir(jobsDir)
	if err != nil {
		logger.Errorf("failed to read jobs folder: %s", err.Error())
		return nil, err
	}
	for _, file := range files {
		var ymlPath string
		if keyword != "" {
			if strings.Contains(file.Name(), keyword) {
				//job yml file path
				ymlPath = getJobFilePath(file.Name())
			} else {
				continue
			}
		} else {
			ymlPath = getJobFilePath(file.Name())
		}
		if !isFileExist(ymlPath) {
			logger.Warnf("job file not exist: %s", ymlPath)
			continue
		}
		fileContent, err := os.ReadFile(ymlPath)
		if err != nil {
			logger.Error("get job read file failed", err.Error())
			continue
		}
		var jobData model.Job
		var jobVo model.JobVo
		//deserialization job yml file
		err = yaml.Unmarshal(fileContent, &jobData)
		if err != nil {
			logger.Error("get job,deserialization job file failed", err.Error())
			continue
		}
		copier.Copy(&jobVo, &jobData)
		updateJobInfo(&jobVo)
		createTime := platform.GetFileCreateTime(ymlPath)
		jobVo.CreateTime = *createTime
		jobs = append(jobs, jobVo)
	}
	sort.Sort(model.JobVoTimeDecrement(jobs))
	pageNum, size, start, end := utils.SlicePage(page, pageSize, len(jobs))
	jobPage.Page = pageNum
	jobPage.PageSize = size
	jobPage.Total = len(jobs)
	jobPage.Data = jobs[start:end]
	return &jobPage, nil
}

// JobDetailList job detail list
func JobDetailList(name string, page, pageSize int) (*model.JobDetailPage, error) {
	var jobDetailPage model.JobDetailPage
	var jobDetails []model.JobDetail
	//get the folder path of job details
	jobDetailDir := getJobDetailFileDir(name)
	if !isFileExist(jobDetailDir) {
		logger.Error("job-details folder does not exist")
		return nil, fmt.Errorf("job-details folder does not exist")
	}
	files, err := os.ReadDir(jobDetailDir)
	if err != nil {
		logger.Error("failed to read jobs folder", err.Error())
		return nil, err
	}
	for _, file := range files {
		ymlPath := filepath.Join(jobDetailDir, file.Name())
		// judge whether the job detail file exists
		if !isFileExist(ymlPath) {
			logger.Error("job detail file not exist")
			continue
		}
		fileContent, err := os.ReadFile(ymlPath)
		if err != nil {
			logger.Error("get job detail read file failed", err.Error())
			continue
		}
		var jobDetailData model.JobDetail
		// deserialization job yml file
		err = yaml.Unmarshal(fileContent, &jobDetailData)
		if err != nil {
			logger.Error("get job detail,deserialization job file failed", err.Error())
			continue
		}
		jobDetails = append(jobDetails, jobDetailData)
	}
	sort.Sort(model.JobDetailDecrement(jobDetails))
	pageNum, size, start, end := utils.SlicePage(page, pageSize, len(jobDetails))
	jobDetailPage.Page = pageNum
	jobDetailPage.PageSize = size
	jobDetailPage.Total = len(jobDetails)
	jobDetailPage.Data = jobDetails[start:end]
	return &jobDetailPage, nil
}

// DeleteJobDetail delete job detail
func DeleteJobDetail(name string, pipelineDetailId int) error {
	// job detail file path
	jobDetailFilePath := GetJobDetailFilePath(name, pipelineDetailId)
	// judge whether the job detail file exists
	if !isFileExist(jobDetailFilePath) {
		logger.Error("delete job detail failed,job detail file not exist")
		return fmt.Errorf("delete job detail failed,job detail file not exist")
	}
	return deleteFile(jobDetailFilePath)
}

// CreateJobDetail exec pipeline job
func CreateJobDetail(name string, id int) (*model.JobDetail, error) {
	jobData, err := GetJobObject(name)
	if err != nil {
		return nil, err
	}
	var jobDetail model.JobDetail
	jobDetailFileDir := getJobDetailFileDir(name)
	err = createDirIfNotExist(jobDetailFileDir)
	if err != nil {
		logger.Error("create job detail file dir failed: ", err.Error())
		return nil, err
	}
	jobDetail.Id = id
	stageDetail, err := jobData.StageSort()
	if err != nil {
		logger.Error("create job detail failed: ", err.Error())
		return nil, err
	}
	jobDetail.Job = *jobData
	jobDetail.Status = model.STATUS_NOTRUN
	jobDetail.StartTime = time.Now()
	jobDetail.Stages = stageDetail
	jobDetail.TriggerMode = consts.TRIGGER_MODE
	return &jobDetail, SaveJobDetail(name, &jobDetail)
}

// GetJobLog 获取 job 日志
func GetJobLog(name string, pipelineDetailId int) (*model.JobLog, error) {
	logPath := getJobDetailLogPath(name, pipelineDetailId)
	fileLog, err := output.ParseLogFile(logPath)
	if err != nil {
		logger.Errorf("parse log file failed, %v", err)
		return nil, err
	}
	jobLog := &model.JobLog{
		StartTime: fileLog.StartTime,
		Duration:  fileLog.Duration,
		Content:   strings.Join(fileLog.Lines, "\r"),
		LastLine:  len(fileLog.Lines),
	}
	return jobLog, nil
}

// GetJobLogString 获取 job 日志字符串
func GetJobLogString(name string, pipelineDetailId int) (string, error) {
	logPath := getJobDetailLogPath(name, pipelineDetailId)
	return readStringFromFile(logPath)
}

// SaveJobLogString 保存 job 日志字符串
func SaveJobLogString(name string, pipelineDetailId int, content string) error {
	logPath := getJobDetailLogPath(name, pipelineDetailId)
	return saveStringToFile(logPath, content)
}

// GetJobStepLog 获取 job 的 step 日志
func GetJobStepLog(name string, id int, stageName, stepName string) (*output.Step, error) {
	logPath := getJobDetailLogPath(name, id)
	fileLog, err := output.ParseLogFile(logPath)
	if err != nil {
		logger.Errorf("parse log file failed, %v", err)
		return nil, err
	}
	for _, stage := range fileLog.Stages {
		if stage.Name == stageName {
			steps := output.ParseStageSteps(&stage)
			for _, step := range steps {
				if step.Name == stepName {
					return step, nil
				}
			}
		}
	}
	return nil, nil
}

// GetJobStageLog 获取 job 的 stage 日志
func GetJobStageLog(name string, execId int, stageName string, start int) (*model.JobStageLog, error) {
	logPath := getJobDetailLogPath(name, execId)
	fileLog, err := output.ParseLogFile(logPath)
	if err != nil {
		logger.Errorf("parse log file failed, %v", err)
		return nil, err
	}

	detail, err := GetJobDetail(name, execId)
	if err != nil {
		return nil, err
	}

	var stageDetail model.StageDetail

	for _, stage := range detail.Stages {
		if stage.Name == stageName {
			stageDetail = stage
		}
	}

	for _, stage := range fileLog.Stages {
		if stage.Name == stageName {
			var content string
			if start >= 0 && start <= len(stage.Lines) {
				content = strings.Join(stage.Lines[start:], "\r")
			}

			return &model.JobStageLog{
				StartTime: stage.StartTime,
				Duration:  stage.Duration,
				Content:   content,
				LastLine:  len(stage.Lines),
				End:       stageDetail.Status == model.STATUS_SUCCESS || stageDetail.Status == model.STATUS_FAIL,
			}, nil
		}
	}
	return nil, fmt.Errorf("stage %s not found", stageName)
}

// 就地更新 job 详情
func updateJobInfo(jobData *model.JobVo) error {
	//get the folder path of job details
	jobDetailDir := getJobDetailFileDir(jobData.Name)
	if !isFileExist(jobDetailDir) {
		logger.Error("job-details folder does not exist")
		return fmt.Errorf("job-details folder does not exist")
	}
	files, err := os.ReadDir(jobDetailDir)
	if err != nil {
		logger.Error("failed to read jobs folder", err.Error())
		return err
	}
	var ids []int
	for _, file := range files {
		index := strings.Index(file.Name(), ".")
		id, err := strconv.Atoi(file.Name()[0:index])
		if err != nil {
			logger.Error("string to int failed", err.Error())
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		sort.Sort(sort.Reverse(sort.IntSlice(ids)))
		jobDetail, err := GetJobDetail(jobData.Name, ids[0])
		if err != nil {
			logger.Errorf("get job detail failed, %s", err)
			return err
		}
		jobData.Duration = jobDetail.Duration
		jobData.Status = jobDetail.Status
		jobData.TriggerMode = jobDetail.TriggerMode
		jobData.StartTime = jobDetail.StartTime
		jobData.TriggerMode = jobDetail.TriggerMode
		jobData.PipelineDetailId = jobDetail.Id
		jobData.Error = jobDetail.Error
	}
	return nil
}

// GetJobObject 获取 job 对象
func GetJobObject(name string) (*model.Job, error) {
	var jobData model.Job
	// job file path
	jobFilePath := getJobFilePath(name)
	if !isFileExist(jobFilePath) {
		logger.Warnf("get job failed, job file not exist: %s", jobFilePath)
		return nil, fmt.Errorf("get job failed, job file not exist: %s", jobFilePath)
	}
	fileContent, err := os.ReadFile(jobFilePath)
	if err != nil {
		logger.Error("get job read file failed: ", err.Error())
		return nil, err
	}
	err = yaml.Unmarshal(fileContent, &jobData)
	if err != nil {
		logger.Error("get job,deserialization job file failed", err.Error())
		return nil, err
	}
	return &jobData, nil
}

// OpenArtifactoryDir open artifactory folder
func OpenArtifactoryDir(name string, detailId string) error {
	artifactoryDir := GetJobArtifactoryDir(name, detailId)
	return platform.OpenDir(artifactoryDir)
}

// GetJobArtifactoryDir 获取 job 的 artifactory 目录路径
func GetJobArtifactoryDir(jobName string, jobID string) string {
	return filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME, jobName, consts.ArtifactoryName, jobID)
}

func GetJobCheckDir(jobName string, jobID string) string {
	return filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME, jobName, consts.CheckName, jobID)
}

func GetJobArtifactoryFiles(jobName string, jobID string) ([]string, error) {
	artifactoryDir := GetJobArtifactoryDir(jobName, jobID)
	return ListFilesAbs(artifactoryDir)
}

func GetJobCheckFiles(jobName string, jobID string) ([]string, error) {
	checkDir := GetJobCheckDir(jobName, jobID)
	return ListFilesAbs(checkDir)
}

func GetJobArtifactoryFilesData(jobName string, jobID string) ([]*api.File, error) {
	files, err := GetJobArtifactoryFiles(jobName, jobID)
	if err != nil {
		return nil, err
	}
	var result []*api.File
	for _, path := range files {
		data, err := GetFileData(path)
		if err != nil {
			return nil, err
		}
		result = append(result, &api.File{
			Path: getRelativeJobsPath(path),
			Data: data,
		})
	}
	return result, nil
}

func GetJobCheckFilesData(jobName string, jobID string) ([]*api.File, error) {
	files, err := GetJobCheckFiles(jobName, jobID)
	if err != nil {
		return nil, err
	}
	var result []*api.File
	for _, path := range files {
		data, err := GetFileData(path)
		if err != nil {
			return nil, err
		}
		result = append(result, &api.File{
			Path: getRelativeJobsPath(path),
			Data: data,
		})
	}
	return result, nil
}

// 获取该路径相对于 jobs 文件夹的路径
func getRelativeJobsPath(path string) string {
	jobsDir := GetJobsDir()
	if strings.HasPrefix(path, jobsDir) {
		return path[len(jobsDir)+1:]
	}
	return path
}

func GetJobsDir() string {
	return filepath.Join(utils.DefaultConfigDir(), consts.JOB_DIR_NAME)
}

// WriteFileToJobsDir 把文件写入到 jobs 文件夹下
func WriteFileToJobsDir(fileName string, content []byte) error {
	jobsDir := GetJobsDir()
	return SaveFile(filepath.Join(jobsDir, fileName), content)
}

// MakeJobStop 使 job 停止
func MakeJobStop(jobName string, jobDetailId int, errorString string) error {
	jobDetail, err := GetJobDetail(jobName, jobDetailId)
	if err != nil {
		return err
	}
	jobDetail.Status = model.STATUS_STOP
	jobDetail.Error = errorString
	return SaveJobDetail(jobName, jobDetail)
}
