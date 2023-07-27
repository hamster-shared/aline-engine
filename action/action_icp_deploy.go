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
	userId  uint
	ac      ctx.ActionContext
}

func NewICPDeployAction(ac ctx.ActionContext) *ICPDeployAction {
	userId := ac.GetStackValue("userId").(uint)

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
	err2 := a.downloadAndUnzip()
	if err2 != nil {
		return nil, err2
	}

	fmt.Println(a.dfxJson)

	err := os.WriteFile(path.Join(workdir, "dfx.json"), []byte(a.dfxJson), 0644)
	if err != nil {
		fmt.Println("write dfx.json error:", err)
		return nil, err
	}

	// 设置默认值
	icNetwork := os.Getenv("IC_NETWORK")
	if icNetwork == "" {
		icNetwork = "local"
	}

	cmd := exec.Command("/usr/local/bin/dfx", "deploy", "--network", icNetwork)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("执行CMD命令时发生错误:", err)
		return nil, err
	}

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

	// 输出结果
	fmt.Println("提取的键值对：")
	for key, value := range keyValuePairs {
		fmt.Printf("%s: %s\n", key, value)
	}

	return keyValuePairs
}

func (a *ICPDeployAction) Post() error {
	//缓存 .dfx 目录

	return nil
}
