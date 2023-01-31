package engine

import (
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestEngine(t *testing.T) {

	logger.Init().ToStdoutAndFile().SetLevel(logrus.TraceLevel)

	engine := NewEngine()

	jobName := "frontend-check"
	yaml := `version: 1.0
name: frontend-check
stages:
  Initialization:
    steps:
      - name: git-clone
        uses: git-checkout
        with:
          url: https://github.com/abing258/frontend-Template.git
          branch: master

  Check FrontEnd :
    needs:
      - Initialization
    steps:
      - name: frontend-install
        run: |
          npm install
      - name: frontend-check
        uses: frontend-check
        with:
          path:

  Output Results:
    needs:
      - Check FrontEnd
    steps:
      - name: check-aggregation
        uses: check-aggregation
`
	err := engine.CreateJob(jobName, yaml)
	assert.NoError(t, err)

	job := engine.GetJob(jobName)
	_, err = job.StageSort()
	assert.NoError(t, err)

	go engine.Start()
	detail, err := engine.ExecuteJob(jobName)

	assert.NoError(t, err)
	fmt.Println(detail.Id)

	time.Sleep(30 * time.Second)

}
