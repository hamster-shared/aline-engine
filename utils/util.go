package utils

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hamster-shared/aline-engine/consts"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func GetUIDAndGID() (string, string, error) {
	u, err := user.Current()
	if err != nil {
		return "", "", err
	}
	return u.Uid, u.Gid, nil
}

type Cmd struct {
	ctx context.Context
	*exec.Cmd
}

// NewCommand is like exec.CommandContext but ensures that subprocesses
// are killed when the context times out, not just the top level process.
func NewCommand(ctx context.Context, command string, args ...string) *Cmd {
	return &Cmd{ctx, exec.Command(command, args...)}
}

func (c *Cmd) Start() error {
	// Force-enable setpgid bit so that we can kill child processes when the
	// context times out or is canceled.
	//if c.Cmd.SysProcAttr == nil {
	//	c.Cmd.SysProcAttr = &syscall.SysProcAttr{}
	//}
	//c.Cmd.SysProcAttr.Setpgid = true
	err := c.Cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		<-c.ctx.Done()
		p := c.Cmd.Process
		if p == nil {
			return
		}
		// Kill by negative PID to kill the process group, which includes
		// the top-level process we spawned as well as any subprocesses
		// it spawned.
		//_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	}()
	return nil
}

func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func DefaultConfigDir() string {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		log.Println("get user home dir failed", err.Error())
		return consts.PIPELINE_DIR_NAME + "."
	}
	dir := filepath.Join(userHomeDir, consts.PIPELINE_DIR_NAME)
	return dir
}

// SlicePage paging   return:page,pageSize,start,end
func SlicePage(page, pageSize, nums int) (int, int, int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize < 0 {
		pageSize = 10
	}
	if pageSize > nums {
		return page, pageSize, 0, nums
	}
	// total page
	pageCount := int(math.Ceil(float64(nums) / float64(pageSize)))
	if page > pageCount {
		return page, pageSize, 0, 0
	}
	sliceStart := (page - 1) * pageSize
	sliceEnd := sliceStart + pageSize

	if sliceEnd > nums {
		sliceEnd = nums
	}
	return page, pageSize, sliceStart, sliceEnd
}

func GetMyIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown", err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "unknown", fmt.Errorf("can not get ip")
}

func GetMyHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown", err
	}
	return hostname, nil
}

func GetNodeKey(name, address string) string {
	return fmt.Sprintf("%s@%s", name, address)
}

// FormatJobToString 格式化为字符串
// return: name(id)
func FormatJobToString(name string, id int) string {
	return fmt.Sprintf("%s(%d)", name, id)
}

func GetJobNameAndIDFromFormatString(str string) (string, int, error) {
	// name(id)
	splitString := strings.Split(str, "(")
	if len(splitString) != 2 {
		return "", 0, fmt.Errorf("format error")
	}
	name := splitString[0]
	id, err := strconv.Atoi(strings.TrimRight(splitString[1], ")"))
	if err != nil {
		return "", 0, err
	}
	return name, id, nil
}
