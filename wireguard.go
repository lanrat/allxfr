package main

import (
	"fmt"
	"net/netip"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

var tnet *netstack.Net

func wg(localIP, dnsIP, privateKey, publicKey, allowedIPs, endpoint string) (*netstack.Net, error) {
	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr(localIP)},
		[]netip.Addr{netip.MustParseAddr(dnsIP)},
		1420)
	if err != nil {
		return nil, err
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, ""))
	conf := fmt.Sprintf("private_key=%s\npublic_key=%s\nallowed_ip=%s\nendpoint=%s", privateKey, publicKey, allowedIPs, endpoint)
	err = dev.IpcSet(conf)
	if err != nil {
		return nil, err
	}
	err = dev.Up()
	if err != nil {
		return nil, err
	}
	return tnet, nil
}
