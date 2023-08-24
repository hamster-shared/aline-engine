package action

import (
	"encoding/json"
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

//**
//**
//**

const MAINNET_CANDID_INTERFACE_PRINCIPAL = "a4gq6-oaaaa-aaaab-qaa4q-cai"

// ICPDeployAction Upload files/directories to ipfs
type ICPDeployAction struct {
	artiUrl   string
	dfxJson   string
	userId    string
	ac        ctx.ActionContext
	deployCmd bool
}

func NewICPDeployAction(ac ctx.ActionContext) *ICPDeployAction {
	userId := ac.GetUserId()
	deployCmd := false
	if ac.GetStepWith("deploy_cmd") == "true" {
		deployCmd = true
	}

	return &ICPDeployAction{
		artiUrl:   ac.GetStepWith("arti_url"),
		dfxJson:   ac.GetStepWith("dfx_json"),
		deployCmd: deployCmd,
		userId:    userId,
		ac:        ac,
	}
}

func (a *ICPDeployAction) Pre() error {
	params := a.ac.GetParameters()
	a.artiUrl = utils.ReplaceWithParam(a.artiUrl, params)
	a.dfxJson = utils.ReplaceWithParam(a.dfxJson, params)

	workdir := a.ac.GetWorkdir()

	// 设置默认值
	icNetwork := os.Getenv("IC_NETWORK")
	if icNetwork == "" {
		icNetwork = "local"
	}
	dfxBin := "/usr/local/bin/dfx"

	var dfxJson DFXJson
	if err := json.Unmarshal([]byte(a.dfxJson), &dfxJson); err != nil {
		return err
	}

	isDepoyed := checkIsDeployed(workdir, icNetwork)

	if !isDepoyed {
		for canisterId, _ := range dfxJson.Canisters {
			cmd := exec.Command(dfxBin, "canister", "create", canisterId, "--network", icNetwork, "--with-cycles", "300000000000")
			logger.Info("execute: ", strings.Join(cmd.Args, " "))
			cmd.Dir = workdir
			logger.Infof("execute create canister command: %s", cmd)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logger.Error("execute command error:", err)
				a.ac.WriteLine(string(output))
				return fmt.Errorf(string(output))
			}
		}
	}

	err2 := a.downloadAndUnzip(icNetwork)
	if err2 != nil {
		return err2
	}

	err := os.WriteFile(path.Join(workdir, "dfx.json"), []byte(a.dfxJson), 0644)
	if err != nil {
		logger.Error("write dfx.json error:", err)
		return err
	}

	locker, err := utils.Lock()
	if err != nil {
		return err
	}

	defer utils.Unlock(locker)

	cmd := exec.Command(dfxBin, "identity", "use", a.userId)
	logger.Info("execute: ", strings.Join(cmd.Args, " "))
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	logger.Info(string(output))
	if err != nil {
		return err
	}

	return nil
}

func checkIsDeployed(workdir string, network string) bool {
	if network == "ic" {
		_, err := os.Stat(path.Join(workdir, CANISTER_IDS_JSON))
		return err == nil
	} else {
		_, err := os.Stat(path.Join(workdir, ".dfx", network, CANISTER_IDS_JSON))
		return err == nil
	}
}

func (a *ICPDeployAction) Hook() (*model.ActionResult, error) {

	workdir := a.ac.GetWorkdir()

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
	logger.Info("execute: ", strings.Join(cmd.Args, " "))
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	logger.Info(string(output))

	actionResult := &model.ActionResult{}
	if a.deployCmd {
		cmd = exec.Command(dfxBin, "deploy", "--network", icNetwork, "--with-cycles", "300000000000")
		logger.Info("execute: ", strings.Join(cmd.Args, " "))
		cmd.Dir = workdir
		logger.Infof("execute deploy canister command: %s", cmd)
		output, err = cmd.CombinedOutput()
		if err != nil {
			logger.Error("execute deploy fail:", err)
			a.ac.WriteLine(string(output))
			return nil, fmt.Errorf(string(output))
		}

		a.ac.WriteLine(string(output))
		logger.Info(string(output))

		urls := analyzeURL(string(output))

		for key, value := range urls {
			actionResult.Deploys = append(actionResult.Deploys, model.DeployInfo{
				Name: key,
				Url:  value,
			})
		}
	} else {
		// 解析dfx.json ，查询出罐名称
		var dfxJson DFXJson

		bytes, _ := os.ReadFile(path.Join(workdir, "dfx.json"))

		if err := json.Unmarshal(bytes, &dfxJson); err != nil {

			return actionResult, err
		}

		for canisterId, _ := range dfxJson.Canisters {
			fmt.Println("canisterId : ", canisterId)
			cmd := exec.Command(dfxBin, "canister", "install", canisterId, "--yes", "--mode=reinstall", "--network", icNetwork)
			logger.Info("execute: ", strings.Join(cmd.Args, " "))
			cmd.Dir = workdir
			output, err = cmd.CombinedOutput()
			logger.Info(string(output))
			canisterType := dfxJson.Canisters[canisterId]["type"]
			var url string
			if canisterType == "assets" {
				url = fmt.Sprintf("https://%s.icp0.io/", canisterId)
			} else {
				url = fmt.Sprintf("https://%s.raw.icp0.io/?id=%s", MAINNET_CANDID_INTERFACE_PRINCIPAL, canisterId)
			}
			if err != nil {
				return nil, err
			}
			actionResult.Deploys = append(actionResult.Deploys, model.DeployInfo{
				Name: canisterId,
				Url:  url,
			})
		}

	}

	return actionResult, nil
}

func (a *ICPDeployAction) downloadAndUnzip(network string) error {
	workdir := a.ac.GetWorkdir()

	// 解析dfx.json ，查询出罐名称
	var dfxJson DFXJson

	bytes := []byte(a.dfxJson)

	if err := json.Unmarshal(bytes, &dfxJson); err != nil {

		return err
	}
	for canisterId, _ := range dfxJson.Canisters {
		canisterType := dfxJson.Canisters[canisterId]["type"]
		if canisterType == "assets" {
			_ = os.RemoveAll(path.Join(workdir, "dist"))
		} else {
			_ = os.RemoveAll(path.Join(workdir, ".dfx", network, "canisters"))
		}
	}

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

type DFXJson struct {
	Canisters map[string]map[string]any
}
