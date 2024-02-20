package utils

import (
	"github.com/hamster-shared/aline-engine/logger"
	"github.com/werf/lockgate"
	"github.com/werf/lockgate/pkg/distributed_locker"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"time"

	"github.com/werf/kubedog/pkg/kube"
)

const (
	ICP_LOCK_KEY = "icp-lock"
	NAMESPACE    = "hamster"
)

var locker *distributed_locker.DistributedLocker

func init() {
	//locker, err := file_locker.NewFileLocker("/tmp/mylock")

	if err := kube.Init(kube.InitOptions{}); err != nil {
		logger.Errorf("cannot initialize kube: %s", err)
		os.Exit(1)
	}

	locker = distributed_locker.NewKubernetesLocker(
		kube.DynamicClient, schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "configmaps",
		}, ICP_LOCK_KEY, NAMESPACE,
	)
}

func Lock() (*lockgate.LockHandle, error) {
	// Case 1: simple blocking lock
	_, lock, err := locker.Acquire(ICP_LOCK_KEY, lockgate.AcquireOptions{Shared: false, Timeout: 30 * time.Second})
	if err != nil {
		logger.Error(os.Stderr, "ERROR: failed to lock myresource: %s\n", err)
		return nil, err
	}
	return &lock, err
}

func Unlock(lock *lockgate.LockHandle) error {
	return locker.Release(*lock)
}
