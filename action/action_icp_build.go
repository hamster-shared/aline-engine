package action

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/action/icp_assert"
	"github.com/hamster-shared/aline-engine/ctx"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

//**
//**
//**

const CANISTER_IDS_JSON = "canister_ids.json"
const DFX_BIN = "/usr/local/bin/dfx"

// ICPBuildAction
type ICPBuildAction struct {
	userId  string
	dfxJson DFXJson
	ac      ctx.ActionContext
}

func NewICPBuildAction(ac ctx.ActionContext) *ICPBuildAction {
	userId := ac.GetUserId()
	params := ac.GetParameters()
	ac.WriteLine(fmt.Sprintf("dfx.json: %s", utils.ReplaceWithParam(ac.GetStepWith("dfx_json"), params)))
	var dfxJson DFXJson
	if err := json.Unmarshal([]byte(utils.ReplaceWithParam(ac.GetStepWith("dfx_json"), params)), &dfxJson); err != nil {
		dfxJson = DFXJson{
			Canisters: map[string]map[string]any{},
		}
	}
	return &ICPBuildAction{
		dfxJson: dfxJson,
		userId:  userId,
		ac:      ac,
	}
}

func (a *ICPBuildAction) Pre() error {

	workdir := a.ac.GetWorkdir()
	_ = os.RemoveAll(path.Join(workdir, ".dfx"))
	return nil
}

func (a *ICPBuildAction) Hook() (*model.ActionResult, error) {

	workdir := a.ac.GetWorkdir()

	// 设置默认值
	icNetwork := os.Getenv("IC_NETWORK")
	if icNetwork == "" {
		icNetwork = "local"
	}

	locker, err := utils.Lock()
	if err != nil {
		return nil, err
	}

	defer utils.Unlock(locker)

	a.ac.WriteLine(fmt.Sprintf("use identity: %s", a.userId))
	cmd := exec.Command(DFX_BIN, "identity", "use", a.userId)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	logger.Info(string(output))
	if err != nil {
		return nil, err
	}

	actionResult := &model.ActionResult{}

	for canisterId := range a.dfxJson.Canisters {
		canisterType := a.dfxJson.Canisters[canisterId]["type"]

		var err error
		switch canisterType {
		case "rust":
			err = a.buildRust(canisterId, icNetwork)
		case "motoko":
			err = a.buildMotoko(canisterId, icNetwork)
		case "custom":
			err = errors.New("unsupport custom now")
		case "assets":
			err = a.buildAsserts(canisterId, icNetwork)
		case "pull":
			err = errors.New("unsupport pull now")
		default:
			err = a.buildMotoko(canisterId, icNetwork)
		}
		if err != nil {
			return nil, err
		}
	}

	// save arti did
	for canisterId := range a.dfxJson.Canisters {
		// check did exists
		didPath := path.Join(workdir, ".dfx", icNetwork, "canisters", canisterId, fmt.Sprintf("%s.did", canisterId))
		if _, err := os.Stat(didPath); err != nil {
			continue
		}
		actionResult.Artifactorys = append(actionResult.Artifactorys, model.Artifactory{
			Name: fmt.Sprintf("%s.did", canisterId),
			Url:  didPath,
		})

	}

	return actionResult, nil
}

func (a *ICPBuildAction) Post() error {
	//缓存 .dfx 目录

	return nil
}

func (a *ICPBuildAction) getDFXVersion() (string, error) {
	cmd := exec.Command(DFX_BIN, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(string(output)), "dfx "), err
}

func (a *ICPBuildAction) buildMotoko(canisterId string, network string) error {

	a.ac.WriteLine("build with motoko")

	workdir := a.ac.GetWorkdir()
	dfxVersion, err := a.getDFXVersion()
	if err != nil {
		return err
	}

	mainPath := a.dfxJson.Canisters[canisterId]["main"].(string)
	if mainPath == "" {
		return errors.New("not found main path")
	}
	_ = os.MkdirAll(path.Join(workdir, ".dfx", network, "canisters", canisterId), os.ModePerm)
	_ = os.MkdirAll(path.Join(workdir, ".dfx", network, "canisters", "idl"), os.ModePerm)

	mocBin := fmt.Sprintf("%s/.cache/dfinity/versions/%s/moc", os.Getenv("HOME"), dfxVersion)
	cmd := exec.Command(mocBin,
		path.Join(workdir, mainPath),
		"-o",
		path.Join(workdir, ".dfx", network, "canisters", canisterId, fmt.Sprintf("%s.wasm", canisterId)),
		"-c",
		"--debug",
		"--idl",
		"--stable-types",
		"--public-metadata",
		"candid:service",
		"--public-metadata",
		"candid:args",
		"--actor-idl",
		path.Join(workdir, ".dfx", network, "canisters", "idl"),
		"--package",
		"base",
		fmt.Sprintf("%s/.cache/dfinity/versions/%s/base", os.Getenv("HOME"), dfxVersion),
	)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	logger.Info(string(output))
	a.ac.WriteLine(string(output))

	return err
}

func (a *ICPBuildAction) buildRust(canisterId string, network string) error {

	a.ac.WriteLine("build with rust")

	workdir := a.ac.GetWorkdir()
	canisterDest := path.Join(workdir, ".dfx", network, "canisters", canisterId)
	_ = os.MkdirAll(canisterDest, os.ModeDir)
	_ = os.MkdirAll(path.Join(workdir, ".dfx", network, "canisters", "idl"), os.ModeDir)

	// cargo build --target wasm32-unknown-unknown --release -p counter --locked
	cmd := exec.Command(path.Join(os.Getenv("HOME"), ".cargo/bin/cargo"),
		"build",
		"--target",
		"wasm32-unknown-unknown",
		"--release",
		"-p",
		canisterId,
		"--locked",
	)

	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	logger.Info(output)
	a.ac.WriteLine(string(output))

	if err != nil {
		return err
	}
	didSrc := path.Join(workdir, a.dfxJson.Canisters[canisterId]["candid"].(string))
	_ = copyFile(didSrc, canisterDest)
	err = copyFile(path.Join(workdir, "target/wasm32-unknown-unknown/release", fmt.Sprintf("%s.wasm", canisterId)), canisterDest)

	return err
}

func (a *ICPBuildAction) buildAsserts(canisterId string, network string) error {
	a.ac.WriteLine("build with assert")

	workdir := a.ac.GetWorkdir()
	canisterDest := path.Join(workdir, ".dfx", network, "canisters", canisterId)
	_ = os.MkdirAll(canisterDest, os.ModeDir)
	_ = os.MkdirAll(path.Join(workdir, ".dfx", network, "canisters", "idl"), os.ModeDir)

	didData := icp_assert.MustAsset("assetstorage.did")
	wasmData := icp_assert.MustAsset("assetstorage.wasm.gz")

	_ = os.WriteFile(path.Join(canisterDest, "assetstorage.did"), didData, os.ModePerm)
	_ = os.WriteFile(path.Join(canisterDest, fmt.Sprintf("%s.wasm.gz", canisterId)), wasmData, os.ModePerm)
	return nil
}

func copyFile(srcPath, destDir string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 获取源文件的信息
	srcFileInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destPath := filepath.Join(destDir, srcFileInfo.Name())

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
