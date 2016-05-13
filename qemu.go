package proxmox

import (
	"errors"
	_ "fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/davecgh/go-spew/spew"
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

type QemuNet map[string]string

type QemuConfig struct {
	Bootdisk string
	Cores    float64
	Digest   string
	Memory   float64
	Net      map[string]QemuNet
	SMBios1  string
	Sockets  float64
	Disks    map[string]string
}

type QemuStatus struct {
	CPU       float64
	CPUs      float64
	Mem       float64
	MaxMem    float64
	Disk      float64
	MaxDisk   float64
	DiskWrite float64
	DiskRead  float64
	NetIn     float64
	NetOut    float64
	Uptime    float64
	QmpStatus string
	Status    string
	Template  string
}

func (qemu QemuVM) Delete() (map[string]interface{}, error) {
	var target string
	var data map[string]interface{}
	var err error

	//fmt.Print("!QemuDelete ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64)
	data, err = qemu.node.proxmox.Delete(target)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func stringToMap(data string, itemSeparator string, kvSeparator string) map[string]string {
	var result map[string]string

	result = make(map[string]string)
	list := strings.Split(data, itemSeparator)
	for _, item := range list {
		kv := strings.Split(item, kvSeparator)
		result[kv[0]] = kv[1]
	}
	return result
}

func (qemu QemuVM) Config() (QemuConfig, error) {
	var target string
	var data map[string]interface{}
	var results map[string]interface{}
	var config QemuConfig
	var err error

	//fmt.Print("!QemuConfig ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/config"
	data, err = qemu.node.proxmox.Get(target)
	results = data["data"].(map[string]interface{})
	if err != nil {
		return config, err
	}
	config = QemuConfig{
		Bootdisk: results["bootdisk"].(string),
		Cores:    results["cores"].(float64),
		Digest:   results["digest"].(string),
		Memory:   results["memory"].(float64),
		Sockets:  results["sockets"].(float64),
		SMBios1:  results["smbios1"].(string),
	}
	disktype := [3]string{"virtio", "sata", "ide"}
	disknum := [4]string{"0", "1", "2", "3"}
	config.Disks = make(map[string]string)
	for _, d := range disktype {
		for _, i := range disknum {
			id := d + i
			if disk, ok := results[id]; ok {
				config.Disks[id] = disk.(string)
			}
		}
	}
	config.Net = make(map[string]QemuNet)
	netnum := [4]string{"0", "1", "2", "3"}
	for _, n := range netnum {
		if net, ok := results["net"+n]; ok {
			config.Net["net"+n] = stringToMap(net.(string), ",", "=")
		}
	}

	return config, nil
}

func (qemu QemuVM) CurrentStatus() (QemuStatus, error) {
	var target string
	var err error
	var data map[string]interface{}
	var results map[string]interface{}
	var status QemuStatus

	//fmt.Println("!QemuStatus ", strconv.FormatFloat(qemu.VMId, 'f', 0, 64))

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/current"
	data, err = qemu.node.proxmox.Get(target)
	results = data["data"].(map[string]interface{})
	if err != nil {
		return status, err
	}
	status = QemuStatus{
		CPU:       results["cpu"].(float64),
		CPUs:      results["cpus"].(float64),
		Mem:       results["mem"].(float64),
		MaxMem:    results["maxmem"].(float64),
		Disk:      results["disk"].(float64),
		MaxDisk:   results["maxdisk"].(float64),
		DiskWrite: results["diskwrite"].(float64),
		DiskRead:  results["diskread"].(float64),
		NetIn:     results["netin"].(float64),
		NetOut:    results["netout"].(float64),
		Uptime:    results["uptime"].(float64),
		QmpStatus: results["qmpstatus"].(string),
		Status:    results["status"].(string),
		Template:  results["template"].(string),
	}
	return status, nil
}

func (qemu QemuVM) WaitForStatus(status string, timeout int) error {
	var i int
	var err error
	var qStatus QemuStatus
	for i = 0; i < timeout; i++ {
		qStatus, err = qemu.CurrentStatus()
		if err != nil {
			return err
		}
		if qStatus.Status == status {
			return nil
		}
		time.Sleep(time.Second * 1)
	}
	return errors.New("Timeout reached")
}

func (qemu QemuVM) Start() error {
	var target string
	var err error

	//fmt.Println("!QemuStart ", strconv.FormatFloat(qemu.VMId, 'f', 0, 64))

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/start"
	_, err = qemu.node.proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Stop() error {
	var target string
	var err error

	//fmt.Print("!QemuStop ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/stop"
	_, err = qemu.node.proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Shutdown() error {
	var target string
	var err error

	//fmt.Print("!QemuShutdown ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/shutdown"
	_, err = qemu.node.proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Suspend() error {
	var target string
	var err error

	//fmt.Print("!QemuSuspend ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/suspend"
	_, err = qemu.node.proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Resume() error {
	var target string
	var err error

	//fmt.Print("!QemuResume ", qemu.VMId)

	target = "nodes/" + qemu.node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/resume"
	_, err = qemu.node.proxmox.Post(target, "")
	return err
}
