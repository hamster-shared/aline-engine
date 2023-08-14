package action

import (
	"encoding/json"
	"fmt"
	"github.com/hamster-shared/aline-engine/ctx"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
	"os"
	"os/exec"
	"path"
)

//**
//**
//**

const CANISTER_IDS_JSON = "canister_ids.json"

// ICPBuildAction
type ICPBuildAction struct {
	dfxJson string
	userId  string
	ac      ctx.ActionContext
}

func NewICPBuildAction(ac ctx.ActionContext) *ICPBuildAction {
	userId := ac.GetUserId()

	return &ICPBuildAction{
		dfxJson: ac.GetStepWith("dfx_json"),
		userId:  userId,
		ac:      ac,
	}
}

func (a *ICPBuildAction) Pre() error {
	params := a.ac.GetParameters()
	a.dfxJson = utils.ReplaceWithParam(a.dfxJson, params)
	return nil
}

func (a *ICPBuildAction) Hook() (*model.ActionResult, error) {

	workdir := a.ac.GetWorkdir()

	err := os.WriteFile(path.Join(workdir, "dfx.json"), []byte(a.dfxJson), 0644)
	if err != nil {
		logger.Error("write dfx.json error:", err)
		return nil, err
	}

	// 解析dfx.json ，查询出罐名称
	var dfxJson DFXJson

	bytes, _ := os.ReadFile(path.Join(workdir, "dfx.json"))

	if err := json.Unmarshal(bytes, &dfxJson); err != nil {
		return &model.ActionResult{}, err
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

	if _, err := os.Stat(path.Join(workdir, CANISTER_IDS_JSON)); err != nil {
		for canisterId, _ := range dfxJson.Canisters {
			cmd = exec.Command(dfxBin, "canister", "create", canisterId, "--network", icNetwork)
			cmd.Dir = workdir
			logger.Infof("execute create canister command: %s", cmd)
			output, err = cmd.CombinedOutput()
			if err != nil {
				logger.Error("execute command error:", err)
				a.ac.WriteLine(string(output))
				return nil, fmt.Errorf(string(output))
			}
		}
	}

	actionResult := &model.ActionResult{}
	cmd = exec.Command(dfxBin, "build", "--network", icNetwork)
	cmd.Dir = workdir
	logger.Infof("execute build canister command: %s", cmd)
	output, err = cmd.CombinedOutput()
	if err != nil {
		logger.Error("execute command error:", err)
		a.ac.WriteLine(string(output))
		return nil, fmt.Errorf(string(output))
	}

	return actionResult, nil
}

func (a *ICPBuildAction) Post() error {
	//缓存 .dfx 目录

	return nil
}
