package run

import (
	"fdocker/cgroups"
	"fdocker/image"
	"fdocker/network"
	"fdocker/utils"
	"fdocker/workdirs"
	"fmt"
	flag "github.com/spf13/pflag"
	"golang.org/x/sys/unix"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

type Executor struct {
}

func New() Executor {
	return Executor{}
}

func (e Executor) CmdName() string {
	return "run"
}

func (e Executor) Implicit() bool {
	return false
}

func (e Executor) Usage() string {
	return "f-docker run [--mem] [--swap] [--pids] [--cpus] <image> <command>"
}

func (e Executor) Exec() {
	runArgs := parseFlags()
	setUpBridge()
	initContainer(runArgs)
}

type runArgs struct {
	mem       int
	swap      int
	pids      int
	cpus      float64
	imageName string
	commands  []string
}

func parseFlags() *runArgs {
	fs := flag.FlagSet{}
	fs.ParseErrorsWhitelist.UnknownFlags = true

	mem := fs.Int("mem", -1, "Max RAM to allow in MB")
	swap := fs.Int("swap", -1, "Max swap to allow in MB")
	pids := fs.Int("pids", -1, "Number of max processes to allow")
	cpus := fs.Float64("cpus", -1, "Number of CPU cores to restrict to")
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Println("Error parsing: ", err)
	}
	if len(fs.Args()) < 2 {
		log.Fatalf("Please pass image name and command to run")
	}
	return &runArgs{
		mem:       *mem,
		swap:      *swap,
		pids:      *pids,
		cpus:      *cpus,
		imageName: fs.Args()[0],
		commands:  fs.Args()[1:],
	}
}

func setUpBridge() {
	accessor := network.GetAccessor()
	// Network Step1: set up fdocker0 bridge on host.
	if ok, err := accessor.IsBridgeSetUp(); !ok || err != nil {
		utils.Must(err)
		log.Println("Setting up the fdocker0 bridge on host...")
		if err := accessor.SetupBridge(); err != nil {
			log.Fatalf("Unable to create fdocker0 bridge: %v", err)
		}
	}
}

func createContainerID() string {
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x",
		randBytes[0], randBytes[1], randBytes[2],
		randBytes[3], randBytes[4], randBytes[5])
}

func getContainerMntPath(containerID string) string {
	return path.Join(workdirs.GetContainerFSHome(containerID), "mnt")
}

func getContainerUpperDirPath(containerID string) string {
	return path.Join(workdirs.GetContainerFSHome(containerID), "upperdir")
}

func getContainerWorkDirPath(containerID string) string {
	return path.Join(workdirs.GetContainerFSHome(containerID), "workdir")
}

func createContainerDirectories(containerID string) {
	contDirs := []string{
		workdirs.GetContainerFSHome(containerID),
		getContainerMntPath(containerID),
		getContainerUpperDirPath(containerID),
		getContainerWorkDirPath(containerID)}
	if err := utils.EnsureDirs(contDirs); err != nil {
		log.Fatalf("Unable to create required directories: %v\n", err)
	}
}

func mountOverlayFileSystem(containerID string, imageShaHex string) {
	var srcLayers []string
	accessor := image.GetAccessor()
	pathManifest := accessor.GetManifestPathForImage(imageShaHex)
	mani := accessor.ParseManifest(pathManifest)
	imageBasePath := accessor.GetBasePathForImage(imageShaHex)
	for _, layer := range mani.Layers {
		srcLayers = append([]string{imageBasePath + "/" + layer[:12] + "/fs"}, srcLayers...)
	}
	contFSHome := workdirs.GetContainerFSHome(containerID)
	mntOptions := "lowerdir=" + strings.Join(srcLayers, ":") + ",upperdir=" + contFSHome + "/upperdir,workdir=" + contFSHome + "/workdir"
	//log.Printf("mntOptions=[%s]", mntOptions)
	//log.Printf("contFSHome mnt=[%s]", contFSHome+"/mnt")
	if err := unix.Mount("none", contFSHome+"/mnt", "overlay", 0, mntOptions); err != nil {
		log.Fatalf("Mount failed: %v\n", err)
	}
}

func unmountNetworkNamespace(containerID string) {
	accessor := network.GetAccessor()
	accessor.UnmountNetworkNamespace(containerID)
}

func unmountContainerFs(containerID string) {
	path.Join(workdirs.ContainersPath(), containerID, "fs", "mnt")
	mountedPath := workdirs.ContainersPath() + "/" + containerID + "/fs/mnt"
	if err := unix.Unmount(mountedPath, 0); err != nil {
		log.Fatalf("Uable to mount container file system: %v at %s", err, mountedPath)
	}
}

func prepareAndExecuteContainer(mem int, swap int, pids int, cpus float64,
	containerID string, imageShaHex string, cmdArgs []string) {

	// Network Step3: Set up the network namespace
	cmd := &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{"/proc/self/exe", "setup-netns", containerID},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	utils.Must(cmd.Run())

	// Network Step4: Set up the virtual interface on namespace
	cmd = &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{"/proc/self/exe", "setup-veth", containerID},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	utils.Must(cmd.Run())
	/*
		From namespaces(7)
		       Namespace Flag            Isolates
		       --------- ----   		 --------
		       Cgroup    CLONE_NEWCGROUP Cgroup root directory
		       IPC       CLONE_NEWIPC    System V IPC,
		                                 POSIX message queues
		       Network   CLONE_NEWNET    Network devices,
		                                 stacks, ports, etc.
		       Mount     CLONE_NEWNS     Mount points
		       PID       CLONE_NEWPID    Process IDs
		       Time      CLONE_NEWTIME   Boot and monotonic
		                                 clocks
		       User      CLONE_NEWUSER   User and group IDs
		       UTS       CLONE_NEWUTS    Hostname and NIS
		                                 domain name
	*/
	var opts []string
	if mem > 0 {
		opts = append(opts, "--mem="+strconv.Itoa(mem))
	}
	if swap >= 0 {
		opts = append(opts, "--swap="+strconv.Itoa(swap))
	}
	if pids > 0 {
		opts = append(opts, "--pids="+strconv.Itoa(pids))
	}
	if cpus > 0 {
		opts = append(opts, "--cpus="+strconv.FormatFloat(cpus, 'f', 1, 64))
	}
	opts = append(opts, "--img="+imageShaHex)
	args := append([]string{containerID}, cmdArgs...)
	args = append(opts, args...)
	args = append([]string{"child-mode"}, args...)
	cmd = exec.Command("/proc/self/exe", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &unix.SysProcAttr{
		Cloneflags: unix.CLONE_NEWPID |
			unix.CLONE_NEWNS |
			unix.CLONE_NEWUTS |
			unix.CLONE_NEWIPC,
	}
	utils.Must(cmd.Run())
}

func initContainer(args *runArgs) {
	mem, swap, pids, cpus, src, cmds := args.mem, args.swap, args.pids, args.cpus, args.imageName, args.commands
	containerID := createContainerID()
	log.Printf("New container ID: %s\n", containerID)
	imgAccessor := image.GetAccessor()
	netAccessor := network.GetAccessor()
	cGroupsAccessor := cgroups.GetAccessor()
	imageShaHex := imgAccessor.DownloadImageIfRequired(src)
	log.Printf("Image to overlay mount: %s\n", imageShaHex)
	createContainerDirectories(containerID)
	mountOverlayFileSystem(containerID, imageShaHex)
	// Network Step2: set up virtual eth connecting from f-docker bridge on host to another virtual eth
	if err := netAccessor.SetupVirtualEthOnHost(containerID); err != nil {
		log.Fatalf("Unable to setup Veth0 on host: %v", err)
	}
	prepareAndExecuteContainer(mem, swap, pids, cpus, containerID, imageShaHex, cmds)
	log.Printf("Container done.\n")
	unmountNetworkNamespace(containerID)
	unmountContainerFs(containerID)
	cGroupsAccessor.RemoveCGroups(containerID)
	_ = os.RemoveAll(workdirs.ContainersPath() + "/" + containerID)
}
