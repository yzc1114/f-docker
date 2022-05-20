package network

import (
	"fdocker/utils"
	"fdocker/workdirs"
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"log"
	"math/rand"
	"net"
	"path"
)

type Accessor struct{}

func GetAccessor() Accessor {
	return Accessor{}
}

/*
	Go through the list of interfaces and return true if the fdocker0 bridge is up
*/

func (n Accessor) IsBridgeSetUp() (bool, error) {
	if links, err := netlink.LinkList(); err != nil {
		log.Printf("Unable to get list of links.\n")
		return false, err
	} else {
		for _, link := range links {
			if link.Type() == "bridge" && link.Attrs().Name == "fdocker0" {
				return true, nil
			}
		}
		return false, err
	}
}

func (n Accessor) SetupBridge() error {
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = "fdocker0"
	fdockerBridge := &netlink.Bridge{LinkAttrs: linkAttrs}
	if err := netlink.LinkAdd(fdockerBridge); err != nil {
		return err
	}
	addr, _ := netlink.ParseAddr("172.31.0.1/16")
	utils.Must(netlink.AddrAdd(fdockerBridge, addr))
	utils.Must(netlink.LinkSetUp(fdockerBridge))
	return nil
}

func (n Accessor) SetupVirtualEthOnHost(containerID string) error {
	veth0 := "veth0_" + containerID[:6]
	veth1 := "veth1_" + containerID[:6]
	linkAttrs := netlink.NewLinkAttrs()
	linkAttrs.Name = veth0
	veth0Struct := &netlink.Veth{
		LinkAttrs:        linkAttrs,
		PeerName:         veth1,
		PeerHardwareAddr: n.createMACAddress(),
	}
	if err := netlink.LinkAdd(veth0Struct); err != nil {
		return err
	}
	utils.Must(netlink.LinkSetUp(veth0Struct))
	fdockerBridge, _ := netlink.LinkByName("fdocker0")
	utils.Must(netlink.LinkSetMaster(veth0Struct, fdockerBridge))
	return nil
}

// SetupContainerNetworkInterface Network Step4: 将虚拟以太网线进行绑定。
func (n Accessor) SetupContainerNetworkInterface(containerID string, pid int) {
	n.setContainerVETHToNewNs(containerID, pid)
	n.setContainerIPAndRoute(containerID, pid)
}

func (n Accessor) setContainerVETHToNewNs(containerID string, pid int) {
	// 获取已经存在的网络命名空间对应的文件夹路径
	//nsMount := n.getNetNsPath(containerID)
	nsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
	//fmt.Printf("nsPath: %s\n", nsPath)

	fd, err := unix.Open(nsPath, unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalf("Unable to open: %v\n", err)
	}
	veth1 := "veth1_" + containerID[:6]
	veth1Link, err := netlink.LinkByName(veth1)
	if err != nil {
		log.Fatalf("Unable to fetch veth1: %v\n", err)
	}
	// 设置这个新的容器的虚拟以太网线到新的命名空间中来。
	if err := netlink.LinkSetNsFd(veth1Link, fd); err != nil {
		log.Fatalf("Unable to set network namespace for veth1: %v\n", err)
	}
}

func (n Accessor) setContainerIPAndRoute(containerID string, pid int) {
	//nsMount := n.getNetNsPath(containerID)
	nsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
	//fmt.Printf("nsPath: %s\n", nsPath)

	fd, err := unix.Open(nsPath, unix.O_RDONLY, 0)
	defer func() {
		_ = unix.Close(fd)
	}()
	if err != nil {
		log.Fatalf("Unable to open: %v\n", err)
	}
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Fatalf("Setns system call failed: %v\n", err)
	}

	veth1 := "veth1_" + containerID[:6]
	veth1Link, err := netlink.LinkByName(veth1)
	if err != nil {
		log.Fatalf("Unable to fetch veth1: %v\n", err)
	}
	addr, _ := netlink.ParseAddr(n.createIPAddress() + "/16")
	// 为这个容器的以太网接口设置ip地址。
	if err := netlink.AddrAdd(veth1Link, addr); err != nil {
		log.Fatalf("Error assigning IP to veth1: %v\n", err)
	}

	// 正式开启这个网络接口设备。相当于命令：ip link set $link up
	utils.MustWithMsg(netlink.LinkSetUp(veth1Link), "Unable to bring up veth1")

	// 为该网络接口设置默认网关。即设置到fdocker0这个网桥的ip地址即可。
	// 设置完以后，即可通过该网关找到其他连接这个网关的容器的ip地址了。
	route := netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: veth1Link.Attrs().Index,
		Gw:        net.ParseIP("172.31.0.1"),
		Dst:       nil,
	}
	utils.MustWithMsg(netlink.RouteAdd(&route), "Unable to add default route")
}

// SetupLocalInterface Network Step5: 设置回环地址。
func (n Accessor) SetupLocalInterface() {
	links, _ := netlink.LinkList()
	for _, link := range links {
		if link.Attrs().Name == "lo" {
			loAddr, _ := netlink.ParseAddr("127.0.0.1/32")
			if err := netlink.AddrAdd(link, loAddr); err != nil {
				log.Println("Unable to configure local interface!")
			}
			netlink.LinkSetUp(link)
		}
	}
}

// SetupNewNetworkNamespace Network Step3: 设置新的命名空间。
func (n Accessor) SetupNewNetworkNamespace(containerID string) {
	_ = utils.EnsureDirs([]string{workdirs.NetNsPath()})
	nsMount := n.getNetNsPath(containerID)
	if _, err := unix.Open(nsMount, unix.O_RDONLY|unix.O_CREAT|unix.O_EXCL, 0644); err != nil {
		log.Fatalf("Unable to open bind mount file: :%v\n", err)
	}

	fd, err := unix.Open("/proc/self/ns/net", unix.O_RDONLY, 0)
	defer unix.Close(fd)
	if err != nil {
		log.Fatalf("Unable to open: %v\n", err)
	}

	// 创建新的网络命名空间，将本进程与原有的命名空间脱离。
	if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
		log.Fatalf("Unshare system call failed: %v\n", err)
	}
	// 使用bind mount的方法，将该命名空间与一个文件进行绑定。即绑定到了{nsMount}这个文件夹当中。
	// 为什么要绑定：因为当一个命名空间的所有进程都退出后，该命名空间就会消失，
	// 然而，如果将该命名空间对应的文件夹进行了bind mount，即可打破这个规定，即使当所有进程都退出，该命名空间依然存在。
	if err := unix.Mount("/proc/self/ns/net", nsMount, "bind", unix.MS_BIND, ""); err != nil {
		log.Fatalf("Mount system call failed: %v\n", err)
	}
}

func (n Accessor) JoinContainerNetworkNamespace(containerID string) error {
	nsMount := n.getNetNsPath(containerID)
	fd, err := unix.Open(nsMount, unix.O_RDONLY, 0)
	if err != nil {
		log.Printf("Unable to open: %v\n", err)
		return err
	}
	if err := unix.Setns(fd, unix.CLONE_NEWNET); err != nil {
		log.Printf("Setns system call failed: %v\n", err)
		return err
	}
	return nil
}

func (n Accessor) createMACAddress() net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)
	hw[0] = 0x02
	hw[1] = 0x42
	rand.Read(hw[2:])
	return hw
}

func (n Accessor) createIPAddress() string {
	byte1 := rand.Intn(254)
	byte2 := rand.Intn(254)
	return fmt.Sprintf("172.31.%d.%d", byte1, byte2)
}

func (n Accessor) UnmountNetworkNamespace(containerID string) {
	netNsPath := n.getNetNsPath(containerID)
	if err := unix.Unmount(netNsPath, 0); err != nil {
		log.Fatalf("Uable to mount network namespace: %v at %s", err, netNsPath)
	}
}

func (n Accessor) getNetNsPath(containerID string) string {
	return path.Join(workdirs.NetNsPath(), containerID)
}
