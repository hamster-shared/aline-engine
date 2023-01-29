package engine

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEngine(t *testing.T) {
	engine := NewEngine()

	jobName := "1_1"
	yaml := `version: 1.0
name: 1_1
stages:

  deploy:
    steps:
      - name: deploy
        run: |
          echo "abc"
          echo "${{ param.ip }}"
`
	err := engine.CreateJob(jobName, yaml)
	assert.NoError(t, err)

	job := engine.GetJob(jobName)
	_, err = job.StageSort()
	assert.NoError(t, err)
}
