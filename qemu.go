package proxmox

import (
	"fmt"
	"strconv"
)

type QemuVM struct {
	Mem       float64
	CPUs      float64
	NetOut    float64
	PID       string
	Disk      float64
	MaxMem    float64
	Status    string
	Template  string
	NetIn     float64
	MaxDisk   float64
	Name      string
	DiskWrite float64
	CPU       float64
	VMId      float64
	DiskRead  float64
	Uptime    float64
	node      Node
}

type QemuList map[string]QemuVM

func (qemu QemuVM) Delete() (map[string]interface{}, error) {
	var target string
	var data map[string]interface{}
	var err error

	fmt.Print("!QemuDelete ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64)
	data, err = qemu.node.proxmox.Delete(target)
	if err != nil {
		return nil, err
	}
	return data, nil
}
