package action

import (
	"fmt"
	shell "github.com/ipfs/go-ipfs-api"
	"os"
	"strings"
	"testing"
)

func TestIpfs(t *testing.T) {
	//sh := shell.NewShell("https://ipfs-console.gke.hamsternet.io")
	//sh := shell.NewShell("/ip4/34.139.20.32/tcp/31203")
	sh := shell.NewShell("/dns/ipfs-console.hamsternet.io/tcp/5001")
	cid, err := sh.Add(strings.NewReader("hello world!"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("added %s\n", cid)

	cid, err = sh.AddDir("/Users/mohaijiang/workdir/781e52d9-eea8-4a44-956d-44fddcd6a4e9_12537/dist")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("added %s\n", cid)
}
