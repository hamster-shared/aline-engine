package action

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRev(t *testing.T) {

	str := `* (HEAD detached at 2cd8bcf) 2cd8bcf9f6ebcbaf6b1df5d90d2700a129b4d7db first commit
  main                       2cd8bcf9f6ebcbaf6b1df5d90d2700a129b4d7db first commit
  remotes/origin/main        2cd8bcf9f6ebcbaf6b1df5d90d2700a129b4d7db first commit`

	fmt.Println(containsBranch(str, "main3"))

}

func TestTruffle(t *testing.T) {
	command := "truffle version"
	commands := strings.Fields(command)
	ctx := context.Background()
	c := exec.CommandContext(ctx, commands[0], commands[1:]...) // mac linux
	//c.Dir = a.workdir
	fmt.Println(os.Environ())

	out, err := c.CombinedOutput()

	if err != nil {
		fmt.Println("err:", err.Error())
	}
	fmt.Println("out:", string(out))

}
