package job

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_renameFile(t *testing.T) {
	err := renameFile("/home/vihv/pipelines/jobs/test/test.yml", "/home/vihv/pipelines/jobs/test1/test1.yml")
	assert.NilError(t, err)
}

func Test_listFiles(t *testing.T) {
	files, err := ListFilesAbs("/home/vihv/pipelines/jobs")
	assert.NilError(t, err)
	for _, f := range files {
		t.Log(f)
	}
}
