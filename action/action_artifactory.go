package action

import (
	"context"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/consts"
	"github.com/hamster-shared/aline-engine/logger"
	model2 "github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"io"
	"os"
	"os/exec"
	path2 "path"
	"path/filepath"
	"strconv"
	"strings"
)

// ArtifactoryAction Storage building
type ArtifactoryAction struct {
	name     string
	path     []string
	compress bool
	output   *output.Output
	ctx      context.Context
}

func NewArtifactoryAction(step model2.Step, ctx context.Context, output *output.Output) *ArtifactoryAction {
	var path string
	s := step.With["path"]
	if s != "" {
		if s[len(s)-1:] == "\n" {
			path = s[:len(s)-1]
		} else {
			path = s
		}
	}

	compress := true

	val, ok := step.With["compress"]
	if ok {
		compress, _ = strconv.ParseBool(val)
	}

	return &ArtifactoryAction{
		name:     step.With["name"],
		path:     strings.Split(path, "\n"),
		ctx:      ctx,
		compress: compress,
		output:   output,
	}
}

func (a *ArtifactoryAction) Pre() error {
	if !(len(a.path) > 0 && a.path[0] != "") {
		return errors.New("the path parameter of the save artifact is required")
	}
	if a.name == "" {
		return errors.New("the name parameter of the save artifact is required")
	}
	stack := a.ctx.Value(STACK).(map[string]interface{})
	workdir, ok := stack["workdir"].(string)
	if !ok {
		return errors.New("get workdir error")
	}
	var fullPathList []string
	for _, path := range a.path {
		absPath := path2.Join(workdir, path)
		fullPathList = append(fullPathList, absPath)
	}
	var absPathList []string
	a.path = GetFiles(workdir, fullPathList, absPathList)
	return nil
}

func (a *ArtifactoryAction) Hook() (*model2.ActionResult, error) {
	a.output.NewStage("artifactory")
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

	// compress
	if a.compress {
		dest := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.ArtifactoryName, jobId, a.name)
		var files []*os.File
		for _, path := range a.path {
			file, err := os.Open(path)
			if err != nil {
				return nil, errors.New("file open fail")
			}
			files = append(files, file)
		}
		err = utils.CompressZip(files, dest)
		if err != nil {
			return nil, errors.New("compression failed")
		}
		logger.Infof("File saved to %s", dest)
		actionResult := model2.ActionResult{
			Artifactorys: []model2.Artifactory{
				{
					Name: a.name,
					Url:  dest,
				},
			},
			Reports: nil,
		}
		return &actionResult, nil
	} else {
		actionResult := &model2.ActionResult{
			Artifactorys: []model2.Artifactory{},
		}
		basePath := path2.Join(userHomeDir, consts.ArtifactoryDir, jobName, consts.ArtifactoryName, jobId)
		os.MkdirAll(basePath, os.ModePerm)
		for _, path := range a.path {
			src, err := os.Open(path)
			if err != nil {
				continue
			}
			dest := path2.Join(basePath, path2.Base(path))
			dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE, os.ModePerm)
			if err != nil {
				continue
			}
			io.Copy(dst, src)
			_ = src.Close()
			_ = dst.Close()

			actionResult.Artifactorys = append(actionResult.Artifactorys, model2.Artifactory{
				Name: path2.Base(dest),
				Url:  dest,
			})
		}
		return actionResult, nil
	}
}

func (a *ArtifactoryAction) Post() error {
	fmt.Println("artifactory Post end")
	return nil
}

func GetFiles(workdir string, fuzzyPath []string, pathList []string) []string {
	files, _ := os.ReadDir(workdir)
	flag := false
	for _, file := range files {
		currentPath := workdir + "/" + file.Name()
		for _, path := range fuzzyPath {
			matched, err := filepath.Match(path, currentPath)
			flag = matched
			if matched && err == nil {
				pathList = append(pathList, currentPath)
			}
		}
		if file.IsDir() && !flag {
			pathList = GetFiles(currentPath, fuzzyPath, pathList)
		}
	}
	return pathList
}

func (a *ArtifactoryAction) ExecuteCommand(commands []string, workdir string) (string, error) {
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
