package action

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	shell "github.com/ipfs/go-ipfs-api"
)

// IpfsAction Upload files/directories to ipfs
type IpfsAction struct {
	path    string
	api     string
	gateway string
	artiUrl string
	baseDir string
	output  *output.Output
	ctx     context.Context
}

func NewIpfsAction(step model.Step, ctx context.Context, output *output.Output) *IpfsAction {
	return &IpfsAction{
		path:    step.With["path"],
		artiUrl: step.With["arti_url"],
		gateway: step.With["gateway"],
		api:     step.With["api"],
		baseDir: step.With["base_dir"],
		ctx:     ctx,
		output:  output,
	}
}

func (a *IpfsAction) Pre() error {
	stack := a.ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)
	a.artiUrl = utils.ReplaceWithParam(a.artiUrl, params)
	a.baseDir = utils.ReplaceWithParam(a.baseDir, params)
	a.gateway = utils.ReplaceWithParam(a.gateway, params)
	return nil
}

func (a *IpfsAction) Hook() (*model.ActionResult, error) {

	stack := a.ctx.Value(STACK).(map[string]interface{})

	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("get workdir error")
	}

	fmt.Println(workdir)

	var downloadFile string

	if a.artiUrl != "" {
		URL, err := url.Parse(a.artiUrl)
		if err != nil {
			a.output.WriteLine("url is invalid")
			return nil, err
		}

		a.output.WriteLine("downloading artifacts")

		if URL.Scheme == "http" || URL.Scheme == "https" {

			res, err := http.Get(a.artiUrl)

			if err != nil {
				a.output.WriteLine("download " + URL.String() + " failed")
				return nil, err
			}
			filename := filepath.Base(a.artiUrl)
			downloadFile = filepath.Join(workdir, filename)
			f, err := os.Create(downloadFile)
			if err != nil {
				a.output.WriteLine("copy file fail")
				return nil, err
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
			a.output.WriteLine("download artifacts success")

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
				a.output.WriteLine("copy file fail")
				return nil, err
			}
			src, err := os.Open(URL.RequestURI())
			defer func(src *os.File) {
				err := src.Close()
				if err != nil {
					logger.Error(err)
				}
			}(src)
			if err != nil {
				a.output.WriteLine("copy file fail")
				return nil, err
			}

			_, _ = io.Copy(f, src)
			a.output.WriteLine("download artifacts success")
		}

		if filepath.Ext(downloadFile) == ".zip" {
			err := utils.DeCompressZip(downloadFile, workdir)
			if err != nil {
				return nil, err
			}
		}
		_ = os.Remove(downloadFile)

		a.path = filepath.Join(workdir)
	}

	if _, err := os.Stat(a.path); err != nil {
		a.output.WriteLine("path or arti_url is empty")
		return nil, err
	}

	sh := shell.NewShell(a.api)
	//cid, err := sh.Add(strings.NewReader("hello world!"))
	cid, err := sh.AddDir(filepath.Join(a.path, a.baseDir))
	if err != nil {
		a.output.WriteLine("error:" + err.Error())
		return nil, err
	}

	deployInfo := model.DeployInfo{
		Cid: cid,
		Url: fmt.Sprintf("%s/ipfs/%s", a.gateway, cid),
	}

	actionResult := &model.ActionResult{}
	actionResult.Deploys = append(actionResult.Deploys, deployInfo)
	return actionResult, nil
}

func (a *IpfsAction) Post() error {
	return nil
}

type IpfsGatewayCloudReq struct {
	UploadID       string `json:"uploadID"`
	UploadFileType string `json:"upload_file_type"`
	UploadType     string `json:"upload_type"`
	Cid            string `json:"cid"`
	Filename       string `json:"filename"`
	ContentType    string `json:"content_type"`
	Size           int    `json:"size"`
	Url            string `json:"url"`
	Status         string `json:"status"`
	Pin            string `json:"pin"`
	Dht            string `json:"dht"`
}
