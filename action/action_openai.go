package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"github.com/sashabaranov/go-openai"
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

type OpenAiChatRequestBody struct {
	Model    string              `json:"model"`
	Messages []OpenAiChatMessage `json:"messages"`
}

type OpenAiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenaiAction struct {
	output *output.Output
	ctx    context.Context
	dir    string
	suffix string
}

func NewOpenaiAction(step model.Step, ctx context.Context, output *output.Output) *OpenaiAction {
	dir := step.With["dir"]
	if dir == "" {
		dir = "contracts"
	}
	suffix := step.With["suffix"]
	if suffix == "" {
		suffix = ".sol"
	}

	return &OpenaiAction{
		ctx:    ctx,
		output: output,
		dir:    dir,
		suffix: suffix,
	}
}

// Pre 执行前准备
func (a *OpenaiAction) Pre() error {

	return nil
}

// Hook 执行
func (a *OpenaiAction) Hook() (*model.ActionResult, error) {

	stack := a.ctx.Value(STACK).(map[string]interface{})
	workdir, _ := stack["workdir"].(string)
	jobId, _ := stack["id"].(string)

	var tmpPaths []string
	files := utils.GetSuffixFiles(path.Join(workdir, a.dir), a.suffix, tmpPaths)

	var checkResult string
	var errData error
	for _, f := range files {
		askResult, err := a.askOpenAiChat(f)
		log.Println(err)
		if err != nil {
			log.Println("*********************")
			errData = err
			break
		}
		checkResult += askResult
	}
	log.Println(checkResult)
	log.Println(errData == nil)
	if checkResult == "" && errData != nil {
		log.Println("--------------------")
		return nil, errData
	}
	log.Println("===========================")
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

	a.output.WriteLine(checkResult)

	return result, nil
}

func (a *OpenaiAction) askOpenAi(file string) string {
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
		logger.Errorf("http.NewRequest,[err=%s][url=%s]", err, url)
		return ""
	}
	request.Header.Set("Connection", "Keep-Alive")
	request.Header.Set("Content-Type", "application/json")
	openAiAPIKEY := os.Getenv("OPENAI_API_KEY")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", openAiAPIKEY))
	var resp *http.Response
	resp, err = http.DefaultClient.Do(request)
	if err != nil {
		logger.Errorf("http.Do failed,[err=%s][url=%s]\n", err, url)
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

func (a *OpenaiAction) askOpenAiChat(file string) (string, error) {
	content, err := os.ReadFile(file)
	prompt := fmt.Sprintf("%s\n### Security risk with above contract", content)
	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: prompt,
			},
		},
	}
	client := getOpenaiClient()
	response, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		fmt.Printf("openai request failed: %v\n", err)
		a.output.WriteLine(fmt.Sprintf("openai request failed: %v\n", err))
		return "", err
	}
	if len(response.Choices) > 0 {
		content := path.Base(file) + " \n "
		for _, choices := range response.Choices {
			content += choices.Message.Content + "\n"
		}
		return content, nil
	}
	return "", nil
}

type OpenAiChatResponseBody struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created uint   `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     uint `json:"prompt_tokens"`
		CompletionTokens uint `json:"completion_tokens"`
		TotalTokens      uint `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index        uint   `json:"index"`
	} `json:"choices"`
}

// Post 执行后清理 (无论执行是否成功，都应该有 Post 的清理)
func (a *OpenaiAction) Post() error {

	return nil
}

func getOpenaiClient() *openai.Client {
	apiKey := os.Getenv("AZURE_API_KEY")
	apiBase := os.Getenv("AZURE_API_BASE")
	deploymentName := os.Getenv("AZURE_DEPLOYMENT_NAME")
	config := openai.DefaultAzureConfig(apiKey, apiBase)
	config.AzureModelMapperFunc = func(model string) string {
		azureModelMapping := map[string]string{
			"gpt-3.5-turbo": deploymentName,
		}
		return azureModelMapping[model]
	}
	return openai.NewClientWithConfig(config)
}
