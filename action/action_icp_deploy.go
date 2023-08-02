package action

import (
	"fmt"
	"github.com/hamster-shared/aline-engine/ctx"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
)

// ICPDeployAction Upload files/directories to ipfs
type ICPDeployAction struct {
	artiUrl string
	dfxJson string
	userId  string
	ac      ctx.ActionContext
}

func NewICPDeployAction(ac ctx.ActionContext) *ICPDeployAction {
	userId := ac.GetUserId()

	return &ICPDeployAction{
		artiUrl: ac.GetStepWith("arti_url"),
		dfxJson: ac.GetStepWith("dfx_json"),
		userId:  userId,
		ac:      ac,
	}
}

func (a *ICPDeployAction) Pre() error {
	params := a.ac.GetParameters()
	a.artiUrl = utils.ReplaceWithParam(a.artiUrl, params)
	a.dfxJson = utils.ReplaceWithParam(a.dfxJson, params)
	return nil
}

func (a *ICPDeployAction) Hook() (*model.ActionResult, error) {

	workdir := a.ac.GetWorkdir()

	_ = os.RemoveAll(path.Join(workdir, "dist"))

	err2 := a.downloadAndUnzip()
	if err2 != nil {
		return nil, err2
	}

	err := os.WriteFile(path.Join(workdir, "dfx.json"), []byte(a.dfxJson), 0644)
	if err != nil {
		logger.Error("write dfx.json error:", err)
		return nil, err
	}

	// 设置默认值
	icNetwork := os.Getenv("IC_NETWORK")
	if icNetwork == "" {
		icNetwork = "local"
	}
	dfxBin := "/usr/local/bin/dfx"

	locker, err := utils.Lock()
	if err != nil {
		return nil, err
	}

	defer utils.Unlock(locker)

	cmd := exec.Command(dfxBin, "identity", "use", a.userId)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	logger.Info(output)

	cmd = exec.Command(dfxBin, "deploy", "--network", icNetwork, "--with-cycles", "300000000000")
	cmd.Dir = workdir
	logger.Infof("execute deploy canister command: %s", cmd)
	output, err = cmd.CombinedOutput()
	if err != nil {
		logger.Error("执行CMD命令时发生错误:", err)
		a.ac.WriteLine(string(output))
		return nil, fmt.Errorf(string(output))
	}

	a.ac.WriteLine(string(output))
	logger.Info(string(output))

	actionResult := &model.ActionResult{}
	urls := analyzeURL(string(output))

	for key, value := range urls {
		actionResult.Deploys = append(actionResult.Deploys, model.DeployInfo{
			Name: key,
			Url:  value,
		})
	}

	return actionResult, nil
}

func (a *ICPDeployAction) downloadAndUnzip() error {
	workdir := a.ac.GetWorkdir()

	var downloadFile string

	if a.artiUrl != "" {
		URL, err := url.Parse(a.artiUrl)
		if err != nil {
			a.ac.WriteLine("url is invalid")
			return err
		}

		a.ac.WriteLine("downloading artifacts")

		if URL.Scheme == "http" || URL.Scheme == "https" {

			res, err := http.Get(a.artiUrl)

			if err != nil {
				a.ac.WriteLine("download " + URL.String() + " failed")
				return err
			}
			filename := filepath.Base(a.artiUrl)
			downloadFile = filepath.Join(workdir, filename)
			f, err := os.Create(downloadFile)
			if err != nil {
				a.ac.WriteLine("copy file fail")
				return err
			}
			_, _ = io.Copy(f, res.Body)
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					logger.Error(err)
				}
			}(res.Body)
			defer func(f *os.File) {
				err := f.Close()
				if err != nil {
					logger.Error(err)
				}
			}(f)
			a.ac.WriteLine("download artifacts success")

		} else if URL.Scheme == "file" {
			filename := filepath.Base(a.artiUrl)
			downloadFile = filepath.Join(workdir, filename)
			f, err := os.Create(downloadFile)
			defer func(f *os.File) {
				err := f.Close()
				if err != nil {
					logger.Error(err)
				}
			}(f)
			if err != nil {
				a.ac.WriteLine("copy file fail")
				return err
			}
			src, err := os.Open(URL.RequestURI())
			defer func(src *os.File) {
				err := src.Close()
				if err != nil {
					logger.Error(err)
				}
			}(src)
			if err != nil {
				a.ac.WriteLine("copy file fail")
				return err
			}

			_, _ = io.Copy(f, src)
			a.ac.WriteLine("download artifacts success")
		}

		if filepath.Ext(downloadFile) == ".zip" {
			err := utils.DeCompressZip(downloadFile, workdir)
			if err != nil {
				return err
			}
		}
		_ = os.Remove(downloadFile)

	}

	return nil
}

func analyzeURL(output string) map[string]string {

	// 定义正则表达式来匹配键值对
	pattern := `([^:\s]+):\s*(https?://[^\s]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(output, -1)

	// 创建map来保存键值对
	keyValuePairs := make(map[string]string)

	// 处理匹配结果并构建键值对
	for _, match := range matches {
		key := strings.TrimSpace(match[1])
		value := strings.TrimSpace(match[2])
		keyValuePairs[key] = value
	}

	return keyValuePairs
}

func (a *ICPDeployAction) Post() error {
	//缓存 .dfx 目录

	return nil
}
