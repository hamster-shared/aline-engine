package action

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	model2 "github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	utils2 "github.com/hamster-shared/aline-engine/utils"
	"os"
	"os/exec"
	"strings"
)

// ShellAction 命令工作
type ShellAction struct {
	command  string
	filename string
	ctx      context.Context
	output   *output.Output
}

func NewShellAction(step model2.Step, ctx context.Context, output *output.Output) *ShellAction {

	return &ShellAction{
		command: step.Run,
		ctx:     ctx,
		output:  output,
	}
}

func (a *ShellAction) Pre() error {

	stack := a.ctx.Value(STACK).(map[string]interface{})

	params := stack["parameter"].(map[string]string)

	data, ok := stack["workdir"]

	var workdir string
	if ok {
		workdir = data.(string)
	} else {
		return errors.New("workdir error")
	}

	workdirTmp := workdir + "_tmp"

	_ = os.MkdirAll(workdirTmp, os.ModePerm)

	a.filename = workdirTmp + "/" + utils2.RandSeq(10) + ".sh"

	command := utils2.ReplaceWithParam(a.command, params)

	content := []byte("#!/bin/sh\nset -ex\n" + command)
	err := os.WriteFile(a.filename, content, os.ModePerm)
	if err != nil {
		logger.Errorf("write tmp file error: %v", err)
	} else {
		logger.Debugf("write tmp file success: %v", a.filename)
		// 读取这个文件看看
		content, err := os.ReadFile(a.filename)
		if err != nil {
			logger.Errorf("read tmp file error: %v", err)
			return err
		}
		logger.Debugf("read tmp file success: %v", string(content))
	}

	return err
}

func (a *ShellAction) Hook() (*model2.ActionResult, error) {

	// a.output.NewStep("shell")

	stack := a.ctx.Value(STACK).(map[string]interface{})

	workdir, ok := stack["workdir"].(string)
	if !ok {
		return nil, errors.New("workdir is empty")
	}
	logger.Infof("shell stack: %v", stack)
	env, _ := stack["env"].([]string)

	commands := []string{"sh", "-c", a.filename}
	val, ok := stack["withEnv"]
	if ok {
		precommand := val.([]string)
		shellCommand := make([]string, len(commands))
		copy(shellCommand, commands)
		commands = append([]string{}, precommand...)
		commands = append(commands, shellCommand...)
	}

	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = workdir
	c.Env = append(env, os.Environ()...)

	logger.Debugf("execute shell command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))

	stdout, err := c.StdoutPipe()
	if err != nil {
		logger.Errorf("stdout error: %v", err)
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		logger.Errorf("stderr error: %v", err)
		return nil, err
	}

	go func() {
		for {
			// 检测到 ctx.Done() 之后停止读取
			<-a.ctx.Done()
			if a.ctx.Err() != nil {
				logger.Errorf("shell command error: %v", a.ctx.Err())
				return
			} else {
				p := c.Process
				if p == nil {
					return
				}
				// Kill by negative PID to kill the process group, which includes
				// the top-level process we spawned as well as any subprocesses
				// it spawned.
				//_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
				logger.Info("shell command killed")
				return
			}
		}
	}()

	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stdoutScanner.Scan() {
			fmt.Println(stdoutScanner.Text())
			a.output.WriteLine(stdoutScanner.Text())
		}
	}()
	go func() {
		for stderrScanner.Scan() {
			fmt.Println(stderrScanner.Text())
			a.output.WriteLine(stderrScanner.Text())
		}
	}()

	err = c.Start()
	if err != nil {
		logger.Errorf("shell command start error: %v", err)
		return nil, err
	}

	err = c.Wait()
	if err != nil {
		logger.Errorf("shell command wait error: %v", err)
		return nil, err
	}

	logger.Info("execute shell command success")
	return nil, err
}

func (a *ShellAction) Post() error {
	return os.Remove(a.filename)
}
