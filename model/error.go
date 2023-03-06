package model

import "fmt"

type SendJobError struct {
	ErrorNode string // 出错的节点是哪个，记下来，删掉它
	JobName   string
	JobID     int
	Err       error
}

func (e *SendJobError) Error() string {
	return fmt.Sprintf("send job %s(%d) error: %s", e.JobName, e.JobID, e.Err)
}
