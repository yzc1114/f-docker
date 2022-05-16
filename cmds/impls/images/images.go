package images

import "fdocker/image"

type Executor struct {
}

func New() Executor {
	return Executor{}
}

func (e Executor) CmdName() string {
	return "images"
}

func (e Executor) Implicit() bool {
	return false
}

func (e Executor) Usage() string {
	return "f-docker images"
}

func (e Executor) Exec() {
	acc := image.GetAccessor()
	acc.PrintAvailableImages()
}
