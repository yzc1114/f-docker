package workdirs

import "path"

func GetContainerFSHome(containerID string) string {
	return path.Join(ContainersPath(), containerID, "fs")
}
