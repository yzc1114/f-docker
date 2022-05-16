package _interface

type CmdExecutor interface {
	CmdName() string
	Implicit() bool
	Usage() string
	Exec()
}
