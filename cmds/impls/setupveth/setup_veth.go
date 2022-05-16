package setupveth

import (
	"fdocker/network"
	"fdocker/utils"
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
	containerID := utils.ParseSingleArg("Please pass container ID to run")
	acc := network.GetAccessor()
	acc.SetupContainerNetworkInterface(containerID)
}
