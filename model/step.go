package model

import "time"

type Step struct {
	Name      string            `yaml:"name,omitempty" json:"name"`
	Id        string            `yaml:"id,omitempty" json:"id"`
	Uses      string            `yaml:"uses,omitempty" json:"uses"`
	With      map[string]string `yaml:"with,omitempty" json:"with"`
	RunsOn    string            `yaml:"runs-on,omitempty" json:"runsOn"`
	Volumes   []string          `yaml:"volumes,omitempty" json:"volumes"`
	Run       string            `yaml:"run,omitempty" json:"run"`
	Status    Status            `yaml:"status,omitempty" json:"status"`
	StartTime time.Time         `yaml:"startTime,omitempty" json:"startTime"`
	Duration  int64             `yaml:"duration,omitempty" json:"duration"`
}
