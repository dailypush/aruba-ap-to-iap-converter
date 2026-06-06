package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/app"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/dhcphelper"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/tftphelper"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "dhcp-helper" {
		runDHCPHelper(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "tftp-helper" {
		runTFTPHelper(os.Args[2:])
		return
	}
	app.Run()
}

func runDHCPHelper(args []string) {
	fs := flag.NewFlagSet("dhcp-helper", flag.ExitOnError)
	iface := fs.String("iface", "", "direct Ethernet interface")
	serverIP := fs.String("server-ip", "", "DHCP server IP")
	leaseIP := fs.String("lease-ip", "", "IP to offer")
	netmask := fs.String("netmask", "255.255.255.0", "subnet mask")
	duration := fs.Int("duration", 900, "helper lifetime in seconds")
	_ = fs.Parse(args)

	err := dhcphelper.Run(dhcphelper.Config{
		Interface: *iface,
		ServerIP:  net.ParseIP(*serverIP),
		LeaseIP:   net.ParseIP(*leaseIP),
		Netmask:   net.ParseIP(*netmask),
		Duration:  time.Duration(*duration) * time.Second,
		Log: func(s string) {
			fmt.Println(time.Now().Format(time.RFC3339), "iap325-converter", version.Display(), s)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTFTPHelper(args []string) {
	fs := flag.NewFlagSet("tftp-helper", flag.ExitOnError)
	root := fs.String("root", "", "TFTP root directory")
	address := fs.String("address", ":69", "UDP listen address")
	duration := fs.Int("duration", 1800, "helper lifetime in seconds")
	_ = fs.Parse(args)

	err := tftphelper.Run(tftphelper.Config{
		Root:     *root,
		Address:  *address,
		Duration: time.Duration(*duration) * time.Second,
		Log: func(s string) {
			fmt.Println(time.Now().Format(time.RFC3339), "tftp-helper", version.Display(), s)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
