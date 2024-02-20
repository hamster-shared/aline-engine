package action

import (
	"encoding/json"
	"fmt"
	"github.com/hamster-shared/aline-engine/ctx"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/utils"
	"os"
	"os/exec"
	"path"
)

const CANISTER_IDS_JSON = "canister_ids.json"
const DFX_BIN = "/usr/local/bin/dfx"

// ICPBuildAction icp build action
type ICPBuildAction struct {
	userId  string
	dfxJson string
	ac      ctx.ActionContext
}

func NewICPBuildAction(ac ctx.ActionContext) *ICPBuildAction {
	userId := ac.GetUserId()
	params := ac.GetParameters()

	dfxJson := utils.ReplaceWithParam(ac.GetStepWith("dfx_json"), params)
	fmt.Println(fmt.Sprintf("dfx.json: %s", utils.ReplaceWithParam(ac.GetStepWith("dfx_json"), params)))
	ac.WriteLine(fmt.Sprintf("dfx.json: %s", utils.ReplaceWithParam(ac.GetStepWith("dfx_json"), params)))
	return &ICPBuildAction{
		dfxJson: dfxJson,
		userId:  userId,
		ac:      ac,
	}
}

func (a *ICPBuildAction) Pre() error {

	workdir := a.ac.GetWorkdir()
	_ = os.RemoveAll(path.Join(workdir, ".dfx"))
	_ = os.WriteFile(path.Join(workdir, "dfx.json"), []byte(a.dfxJson), os.ModePerm)
	return nil
}

func (a *ICPBuildAction) Hook() (*model.ActionResult, error) {

	workdir := a.ac.GetWorkdir()

	// 设置默认值
	icNetwork := os.Getenv("IC_NETWORK")
	if icNetwork == "" {
		icNetwork = "local"
	}

	//locker, err := utils.Lock()
	//if err != nil {
	//	return nil, err
	//}
	//
	//defer utils.Unlock(locker)

	//a.ac.WriteLine(fmt.Sprintf("use identity: %s", a.userId))
	//cmd := exec.Command(DFX_BIN, "identity", "use", a.userId)
	//cmd.Dir = workdir
	//output, err := cmd.CombinedOutput()
	//a.ac.WriteLine(string(output))
	//if err != nil {
	//	return nil, err
	//}

	actionResult := &model.ActionResult{}

	cmd := exec.Command(DFX_BIN, "build", "--check", "--network", icNetwork, "--identity", a.userId)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	a.ac.WriteLine(string(output))
	if err != nil {
		return nil, err
	}
	var dfxJson DFXJson
	if err := json.Unmarshal([]byte(a.dfxJson), &dfxJson); err != nil {
		dfxJson = DFXJson{
			Canisters: map[string]map[string]any{},
		}
	}
	if err != nil {
		return actionResult, nil
	}
	// save arti did
	for canisterId := range dfxJson.Canisters {
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
