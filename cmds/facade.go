package cmds

import (
	"fdocker/cmds/impls/childmode"
	"fdocker/cmds/impls/images"
	"fdocker/cmds/impls/ps"
	"fdocker/cmds/impls/rmi"
	"fdocker/cmds/impls/run"
	"fdocker/cmds/impls/setupnetns"
	"fdocker/cmds/impls/setupveth"
	cmdsinterface "fdocker/cmds/interface"
	"sort"
)

func GetCmdExecutors() map[string]cmdsinterface.CmdExecutor {
	executors := getCmdExecutorList()
	m := make(map[string]cmdsinterface.CmdExecutor)
	for _, exec := range executors {
		m[exec.CmdName()] = exec
	}
	return m
}

func getCmdExecutorList() []cmdsinterface.CmdExecutor {
	executors := []cmdsinterface.CmdExecutor{
		childmode.New(),
		images.New(),
		ps.New(),
		rmi.New(),
		run.New(),
		setupnetns.New(),
		setupveth.New(),
	}
	sort.Slice(executors, func(i, j int) bool {
		return executors[i].CmdName() < executors[i].CmdName()
	})
	return executors
}

func Usage() []string {
	executors := getCmdExecutorList()
	usages := make([]string, 0, len(executors))
	for _, exec := range executors {
		if !exec.Implicit() {
			usages = append(usages, exec.Usage())
		}
	}
	return usages
}
