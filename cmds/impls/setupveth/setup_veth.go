package setupveth

import (
	"fdocker/network"
	"fdocker/utils"
	"strconv"
)

type Executor struct {
}

func New() Executor {
	return Executor{}
}

func (e Executor) CmdName() string {
	return "setup-veth"
}

func (e Executor) Implicit() bool {
	return true
}

func (e Executor) Usage() string {
	return ""
}

func (e Executor) Exec() {
	args := utils.ParseArgs("Please pass container ID and pid to run")
	containerID, pidStr := args[0], args[1]
	pid, _ := strconv.Atoi(pidStr)
	acc := network.GetAccessor()
	acc.SetupContainerNetworkInterface(containerID, pid)
}
