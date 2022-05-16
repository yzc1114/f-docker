package rmi

import (
	"fdocker/cmds/impls/ps"
	"fdocker/image"
	"fdocker/utils"
	"log"
)

type Executor struct {
}

func New() Executor {
	return Executor{}
}

func (e Executor) CmdName() string {
	return "rmi"
}

func (e Executor) Implicit() bool {
	return false
}

func (e Executor) Usage() string {
	return "f-docker rmi <image-id>"
}

func (e Executor) Exec() {
	imageHash := utils.ParseSingleArg("Please pass image ID to run")
	DeleteImageByHash(imageHash)
}

func DeleteImageByHash(imageShaHex string) {
	accessor := image.GetAccessor()
	imgName, imgTag := accessor.GetImageAndTagByHash(imageShaHex)
	if len(imgName) == 0 {
		log.Fatalf("No such image")
	}
	containers, err := ps.GetRunningContainers()
	if err != nil {
		log.Fatalf("Unable to get running containers list: %v\n", err)
	}
	for _, container := range containers {
		if container.Image == imgName+":"+imgTag {
			log.Fatalf("Cannot delete image becuase it is in use by: %s",
				container.ContainerId)
		}
	}
	accessor.DeleteImageByHash(imageShaHex)
}
