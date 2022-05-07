package ps

import (
	"bufio"
	"fmt"
	"github.com/shuveb/containers-the-hard-way/image"
	"github.com/shuveb/containers-the-hard-way/workdirs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Executor struct {}

func (e Executor) CmdName() string {
	return "ps"
}

func (e Executor) Usage() string {
	return "f-docker ps"
}

func (e Executor) Exec() {
	printRunningContainers()
}

func (e Executor) Implicit() bool {
	return false
}

func New() Executor {
	return Executor{}
}

type RunningContainerInfo struct {
	ContainerId string
	Image   string
	Command string
	PID     int
}

func GetRunningContainers() ([]RunningContainerInfo, error) {
	var containers []RunningContainerInfo
	basePath := "/sys/fs/cgroup/cpu/fdocker"

	entries, err := ioutil.ReadDir(basePath)
	if os.IsNotExist(err) {
		return containers, nil
	} else {
		if err != nil {
			return nil, err
		} else {
			for _, entry := range entries {
				if entry.IsDir() {
					container, _ := getRunningContainerInfoForId(entry.Name())
					if container.PID > 0 {
						containers = append(containers, container)
					}
				}
			}
			return containers, nil
		}
	}
}

/*
	This isn't a great implementation and can possibly be simplified
	using regex. But for now, here we are. This function gets the
	current mount points, figures out which Image is mounted for a
	given container ID, looks it up in our images database which we
	maintain and returns the Image and tag information.
*/

func getDistribution(containerID string) (string, error) {
	var lines []string
	file, err := os.Open("/proc/mounts")
	if err != nil {
		fmt.Println("Unable to read /proc/mounts")
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	accessor := image.GetAccessor()

	for _, line := range lines {
		if strings.Contains(line, containerID) {
			parts := strings.Split(line, " ")
			for _, part := range parts {
				if strings.Contains(part, "lowerdir=") {
					options := strings.Split(part, ",")
					for _, option := range options {
						if strings.Contains(option, "lowerdir=") {
							imagesPath := workdirs.ImagesPath()
							leaderString := "lowerdir=" + imagesPath + "/"
							trailerString := option[len(leaderString):]
							imageID := trailerString[:12]
							img, tag := accessor.GetImageAndTagByHash(imageID)
							return fmt.Sprintf("%s:%s", img, tag), nil
						}
					}
				}
			}
		}
	}
	return "", nil
}

func getRunningContainerInfoForId(containerID string) (RunningContainerInfo, error) {
	container := RunningContainerInfo{}
	var procs []string
	basePath := "/sys/fs/cgroup/cpu/fdocker"

	file, err := os.Open(basePath + "/" + containerID + "/cgroup.procs")
	if err != nil {
		fmt.Println("Unable to read cgroup.procs")
		return container, err
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		procs = append(procs, scanner.Text())
	}
	if len(procs) > 0 {
		pid, err := strconv.Atoi(procs[len(procs)-1])
		if err != nil {
			fmt.Println("Unable to read PID")
			return container, err
		}
		cmd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
		containerMntPath := workdirs.ContainersPath() + "/" + containerID + "/fs/mnt"
		realContainerMntPath, err := filepath.EvalSymlinks(containerMntPath)
		if err != nil {
			fmt.Println("Unable to resolve path")
			return container, err
		}

		if err != nil {
			fmt.Println("Unable to read Command link.")
			return container, err
		}
		img, _ := getDistribution(containerID)
		container = RunningContainerInfo{
			ContainerId: containerID,
			Image:       img,
			Command:     cmd[len(realContainerMntPath):],
			PID:         pid,
		}
	}
	return container, nil
}

/*
		Get the list of running container IDs.

		Implementation logic:
		- Fdocker creates multiple folders in the /sys/fs/cgroup hierarchy
		- For example, for setting cpu limits, fdocker uses /sys/fs/cgroup/cpu/fdocker
	- Inside that folder are folders one each for currently running containers
	- Those folder names are the container IDs we create.
	- getContainerInfoForId() does more work. It gathers more information about running
		containers. See struct RunningContainerInfo for details.
	- Inside each of those folders is a "cgroup.procs" file that has the list
		of PIDs of processes inside of that container. From the PID, we can
		get the mounted path from which the process was started. From that
		mounted path, we can get the Image of the containers since containers
		are mounted via the overlay file system.
*/
func printRunningContainers() {
	containers, err := GetRunningContainers()
	if err != nil {
		os.Exit(1)
	}

	fmt.Println("CONTAINER ID\tIMAGE\t\tCOMMAND")
	for _, container := range containers {
		fmt.Printf("%s\t%s\t%s\n", container.ContainerId, container.Image, container.Command)
	}
}