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
	Cid string `json:"cid"`
	Url string `json:"url"`
}

type ActionResult struct {
	CodeInfo     string
	Artifactorys []Artifactory `json:"artifactorys"`
	Reports      []Report      `json:"reports"`
	Deploys      []DeployInfo  `json:"deploys"`
}
