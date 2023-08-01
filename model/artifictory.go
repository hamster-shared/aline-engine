package model

/*
Artifactory 构建物
*/
type Artifactory struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

/*
Report 构建物报告
*/
type Report struct {
	Id      int    `json:"id"`
	Url     string `json:"url"`
	Type    int    `json:"type"` // 2 合约检查，前端检查  3 openai 报告
	Content string `json:"content"`
}

type DeployInfo struct {
	Name string `json:"name"`
	Cid  string `json:"cid"`
	Url  string `json:"url"`
}

type BuildInfo struct {
	ImageName string `json:"imageName"`
}

type MetaScanReport struct {
	Total          int64  `json:"total"`
	CheckResult    string `json:"checkResult"`
	ResultOverview string `json:"resultOverview"`
	Tool           string `json:"tool""`
}

type ActionResult struct {
	CodeInfo     string
	Artifactorys []Artifactory    `json:"artifactorys"`
	Reports      []Report         `json:"reports"`
	Deploys      []DeployInfo     `json:"deploys"`
	BuildData    []BuildInfo      `json:"buildData"`
	MetaScanData []MetaScanReport `json:"metaScanData"`
}
