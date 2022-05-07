package workdirs

import "github.com/shuveb/containers-the-hard-way/utils"

const FDHomePath = "/var/lib/f-docker"
const FDTempPath = FDHomePath + "/tmp"
const FDImagesPath = FDHomePath + "/images"
const FDContainersPath = "/var/run/f-docker/containers"
const FDNetNsPath = "/var/run/f-docker/net-ns"

func Init() error {
	dirs := []string {FDHomePath, FDTempPath, FDImagesPath, FDContainersPath}
	return utils.EnsureDirs(dirs)
}

func ImagesPath() string {
	return FDImagesPath
}

func TempPath() string {
	return FDTempPath
}

func ContainersPath() string {
	return FDContainersPath
}

func NetNsPath() string {
	return FDNetNsPath
}