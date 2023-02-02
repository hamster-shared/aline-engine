package action

import (
	"context"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	shell "github.com/ipfs/go-ipfs-api"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	return nil
}

func (a *IpfsAction) Hook() (*model.ActionResult, error) {

	stack := a.ctx.Value(STACK).(map[string]interface{})

	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("get workdir error")
	}

	fmt.Println(workdir)

	if a.artiUrl != "" {
		a.output.WriteLine("downloading artifacts")
		res, err := http.Get(a.artiUrl)

		if err != nil {
			panic(err)
		}
		filename := filepath.Base(a.artiUrl)

		fmt.Println(filename)
		fmt.Println(res.Status)
		downloadFile := filepath.Join(workdir, filename)
		f, err := os.Create(filepath.Join(workdir, filename))
		if err != nil {
			panic(err)
		}
		io.Copy(f, res.Body)
		defer res.Body.Close()
		a.output.WriteLine("download artifacts success")

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
		fmt.Fprintf(os.Stderr, "error: %s", err)
		os.Exit(1)
	}
	fmt.Println(cid)
	fmt.Println(fmt.Sprintf("%s/ipfs/%s", a.gateway, cid))
	return nil, nil
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
