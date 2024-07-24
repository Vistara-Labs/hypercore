package main

import (
	"context"
	"github.com/containernetworking/cni/libcni"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"os"
	"runtime"
)

func main() {
	cniPlugin := libcni.NewCNIConfig([]string{
		"/opt/cni/bin",
	}, nil)

	networkConf, err := libcni.LoadConfList("/etc/cni/conf.d", "fcnet")
	if err != nil {
		panic(err)
	}

	runtimeConf := &libcni.RuntimeConf{
		ContainerID: uuid.NewString(),
		NetNS:       "/tmp/netns",
		IfName:      "veth0",
	}

	fd, err := os.OpenFile(runtimeConf.NetNS, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(err)
	}
	fd.Close()

	doneCh := make(chan error)
	go func() {
		defer close(doneCh)
		runtime.LockOSThread()

		err := unix.Unshare(unix.CLONE_NEWNET)
		if err != nil {
			doneCh <- err
			return
		}

		err = unix.Mount("/proc/thread-self/ns/net", runtimeConf.NetNS, "none", unix.MS_BIND, "none")
		if err != nil {
			doneCh <- err
			return
		}
	}()
	err = <-doneCh

	if err != nil {
		panic(err)
	}

	cniResult, err := cniPlugin.AddNetworkList(context.Background(), networkConf, runtimeConf)
	if err != nil {
		panic(err)
	}

	cniResult.Print()
}
