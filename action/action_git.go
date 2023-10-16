package action

import (
	"bufio"
	"context"
	"fmt"
	"github.com/hamster-shared/aline-engine/logger"
	model2 "github.com/hamster-shared/aline-engine/model"
	"github.com/hamster-shared/aline-engine/output"
	"github.com/hamster-shared/aline-engine/utils"
	"os"
	"os/exec"
	"strings"
)

// GitAction git clone
type GitAction struct {
	repository string
	branch     string
	workdir    string
	output     *output.Output
	ctx        context.Context
}

func NewGitAction(step model2.Step, ctx context.Context, output *output.Output) *GitAction {

	stack := ctx.Value(STACK).(map[string]interface{})
	params := stack["parameter"].(map[string]string)

	return &GitAction{
		repository: utils.ReplaceWithParam(step.With["url"], params),
		branch:     utils.ReplaceWithParam(step.With["branch"], params),
		ctx:        ctx,
		output:     output,
	}
}

func (a *GitAction) Pre() error {
	// a.output.NewStep("git")

	stack := a.ctx.Value(STACK).(map[string]interface{})
	a.workdir = stack["workdir"].(string)

	_, err := os.Stat(a.workdir)
	if err != nil {
		err = os.MkdirAll(a.workdir, os.ModePerm)
		if err != nil {
			return err
		}

		command := "git init " + a.workdir
		_, err = a.ExecuteStringCommand(command)
		if err != nil {
			return err
		}
	}

	//TODO ... 检验 git 命令是否存在
	return nil
}

func (a *GitAction) Hook() (*model2.ActionResult, error) {

	stack := a.ctx.Value(STACK).(map[string]interface{})

	logger.Infof("git stack: %v", stack)

	// before
	//commands := []string{"git", "clone", "--progress", a.repository, "-b", a.branch, pipelineName}
	//err := a.ExecuteCommand(commands)
	//if err != nil {
	//	return nil, err
	//}

	command := "git rev-parse --is-inside-work-tree"
	_, err := a.ExecuteStringCommand(command)
	if err != nil {

		command = "git init"
		_, err = a.ExecuteStringCommand(command)
		if err != nil {
			return nil, err
		}

	}

	command = "git config remote.origin.url  " + a.repository
	_, err = a.ExecuteStringCommand(command)
	if err != nil {
		return nil, err
	}

	command = "git config --add remote.origin.fetch +refs/heads/*:refs/remotes/origin/*"
	_, err = a.ExecuteStringCommand(command)
	if err != nil {
		return nil, err
	}

	command = "git config remote.origin.url " + a.repository
	_, err = a.ExecuteStringCommand(command)
	if err != nil {
		return nil, err
	}

	command = "git fetch --tags --progress " + a.repository + " +refs/heads/*:refs/remotes/origin/*"
	_, err = a.ExecuteStringCommand(command)
	if err != nil {
		return nil, err
	}

	command = fmt.Sprintf("git rev-parse refs/remotes/origin/%s^{commit}", a.branch)
	commitId, err := a.ExecuteCommandDirect(strings.Fields(command))
	if err != nil {
		return nil, err
	}

	//dateCommand := "git --no-pager log --pretty=format:“%cd” --date=format:'%b %e %Y' " + commitId
	dateCommand := []string{"git", "--no-pager", "log", "--pretty=format:“%cd”", "--date=format:'%b %e %Y'", "-1"}
	dateCommand = append(dateCommand, commitId)
	commitDate, err := a.ExecuteCommandDirect(dateCommand)
	if err != nil {
		return nil, err
	}
	messageCommand := "git --no-pager log --pretty=format:“%s” -1 " + commitId
	commitMessage, err := a.ExecuteCommandDirect(strings.Fields(messageCommand))
	if err != nil {
		return nil, err
	}

	command = "git config core.sparsecheckout "
	_, _ = a.ExecuteStringCommand(command)

	command = fmt.Sprintf("git checkout -f %s", commitId)
	_, err = a.ExecuteStringCommand(command)
	if err != nil {
		return nil, err
	}

	command = "git branch -a -v --no-abbrev"
	out, err := a.ExecuteCommandDirect(strings.Fields(command))
	if err != nil {
		return nil, err
	}

	if containsBranch(out, a.branch) {
		command = fmt.Sprintf("git branch -D  %s", a.branch)
		out, err = a.ExecuteStringCommand(command)
		logger.Debug(out)
		if err != nil {
			return nil, err
		}
	}

	command = fmt.Sprintf("git checkout -b %s %s", a.branch, commitId)
	out, err = a.ExecuteStringCommand(command)
	a.output.WriteLine(out)
	if err != nil {
		return nil, err
	}

	stack["workdir"] = a.workdir
	return &model2.ActionResult{
		CodeInfo: model2.CodeInfo{
			Branch:        a.branch,
			CommitId:      commitId[0:6],
			CommitDate:    strings.ReplaceAll(commitDate, `"`, ""),
			CommitMessage: strings.ReplaceAll(commitMessage, `"`, ""),
		},
	}, nil
}

func (a *GitAction) Post() error {
	//return os.Remove(a.workdir)
	return nil
}

func (a *GitAction) ExecuteStringCommand(command string) (string, error) {
	commands := strings.Fields(command)
	return a.ExecuteCommand(commands)
}

func (a *GitAction) ExecuteCommand(commands []string) (string, error) {

	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = a.workdir
	logger.Debugf("execute git clone command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))

	stdout, err := c.StdoutPipe()
	if err != nil {
		logger.Errorf("stdout error: %v", err)
		return "nil", err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		logger.Errorf("stderr error: %v", err)
		return "nil", err
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
		return "nil", err
	}

	err = c.Wait()
	if err != nil {
		logger.Errorf("shell command wait error: %v", err)
		return "nil", err
	}

	defer stdout.Close()
	defer stderr.Close()
	return "", err

}

func (a *GitAction) ExecuteCommandDirect(commands []string) (string, error) {

	c := exec.CommandContext(a.ctx, commands[0], commands[1:]...) // mac linux
	c.Dir = a.workdir
	logger.Debugf("execute git clone command: %s", strings.Join(commands, " "))
	a.output.WriteCommandLine(strings.Join(commands, " "))

	out, err := c.CombinedOutput()
	a.output.WriteCommandLine(string(out))
	if err != nil {
		a.output.WriteLine(err.Error())
	}
	return string(out), err

}

func containsBranch(branchOutput, branch string) bool {
	array := strings.Split(branchOutput, "\n")

	for _, s := range array {
		if len(strings.Fields(s)) == 0 {
			continue
		}
		if strings.EqualFold(strings.Fields(s)[0], branch) {
			return true
		}
	}
	return false
}
