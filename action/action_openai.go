package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
)

type OpenAiRequestBody struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	Temperature      uint     `json:"temperature"`
	MaxTokens        uint     `json:"max_tokens"`
	TopP             float32  `json:"top_p"`
	FrequencyPenalty float32  `json:"frequency_penalty"`
	PresencePenalty  float32  `json:"presence_penalty"`
	Stop             []string `json:"stop"`
}

type OpenAiResponseBody struct {
	Id      string    `json:"id"`
	Object  string    `json:"object"`
	Created uint      `json:"created"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
	Usage   Usage     `json:"usage"`
}

type Choices struct {
	Text         string `json:"text"`
	Index        uint   `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     uint `json:"prompt_tokens"`
	CompletionTokens uint `json:"completion_tokens"`
	TotalTokens      uint `json:"total_tokens"`
}

type OpenaiAction struct {
	output *output.Output
	ctx    context.Context
}

func NewOpenaiAction(step model.Step, ctx context.Context, output *output.Output) *OpenaiAction {

	return &OpenaiAction{
		ctx:    ctx,
		output: output,
	}
}

// Pre 执行前准备
func (a *OpenaiAction) Pre() error {

	return nil
}

/*

#            echo curl https://api.openai.com/v1/completions \
#              -H "Content-Type: application/json" \
#              -H "Authorization: Bearer $OPENAI_API_KEY" \
#              -d '{
#              "model": "code-davinci-002",
#              "prompt": "'$content'",
#              "temperature": 0,
#              "max_tokens": 100,
#              "top_p": 1.0,
#              "frequency_penalty": 0.2,
#              "presence_penalty": 0.0,
#              "stop": ["###"]
#            }'
*/

// Hook 执行
func (a *OpenaiAction) Hook() (*model.ActionResult, error) {

	stack := a.ctx.Value(STACK).(map[string]interface{})
	workdir, _ := stack["workdir"].(string)
	jobId, _ := stack["id"].(string)

	var tmpPaths []string
	files := utils.GetSuffixFiles(path.Join(workdir, "contracts"), ".sol", tmpPaths)

	var checkResult string
	for _, f := range files {
		askResult := askOpenAi(f)
		checkResult += askResult
	}

	id, _ := strconv.Atoi(jobId)

	result := &model.ActionResult{
		Reports: []model.Report{
			{
				Id:      id,
				Type:    3,
				Content: checkResult,
			},
		},
	}

	fmt.Println(checkResult)

	return result, nil
}

func askOpenAi(file string) string {
	content, err := os.ReadFile(file)

	prompt := fmt.Sprintf("%s\n### Security risk with above code", content)

	apiReq := OpenAiRequestBody{
		Model:            "code-davinci-002",
		Prompt:           prompt,
		Temperature:      0,
		MaxTokens:        200,
		TopP:             1.0,
		FrequencyPenalty: 0.2,
		PresencePenalty:  0.0,
		Stop:             []string{"###"},
	}
	json_data, err := json.Marshal(apiReq)
	bodyReader := bytes.NewReader(json_data)
	url := "https://api.openai.com/v1/completions"

	request, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		log.Println("http.NewRequest,[err=%s][url=%s]", err, url)
		return ""
	}
	request.Header.Set("Connection", "Keep-Alive")
	request.Header.Set("Content-Type", "application/json")
	openAiAPIKEY := os.Getenv("OPENAI_API_KEY")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", openAiAPIKEY))
	var resp *http.Response
	resp, err = http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("http.Do failed,[err=%s][url=%s]\n", err, url)
		return ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return ""
	}

	b, err := io.ReadAll(resp.Body)

	var apResponse OpenAiResponseBody
	_ = json.Unmarshal(b, &apResponse)

	if len(apResponse.Choices) > 0 {
		return path.Base(file) + " \n " + apResponse.Choices[0].Text + "\n"
	}
	return ""
}

// Post 执行后清理 (无论执行是否成功，都应该有Post的清理)
func (a *OpenaiAction) Post() error {

	return nil
}
