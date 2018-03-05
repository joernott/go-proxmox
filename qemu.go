package proxmox

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type QemuVM struct {
	Mem       float64 `json:"mem"`
	CPUs      float64 `json:"cpus"`
	NetOut    float64 `json:"netout"`
	PID       string  `json:"pid"`
	Disk      float64 `json:"disk"`
	MaxMem    float64 `json:"maxmem"`
	Status    string  `json:"status"`
	Template  float64 `json:"template"`
	NetIn     float64 `json:"netin"`
	MaxDisk   float64 `json:"maxdisk"`
	Name      string  `json:"name"`
	DiskWrite float64 `json:"diskwrite"`
	CPU       float64 `json:"cpu"`
	VMId      float64 `json:"vmid"`
	DiskRead  float64 `json:"diskread"`
	Uptime    float64 `json:"uptime"`
	Node      Node
}

type QemuList map[string]QemuVM

type QemuNet map[string]string

type QemuConfig struct {
	Bootdisk    string  `json:"bootdisk"`
	Cores       float64 `json:"cores"`
	Digest      string  `json:"digest"`
	Memory      float64 `json:"memory"`
	Net         map[string]QemuNet
	SMBios1     string            `json:"smbios1"`
	Sockets     float64           `json:"sockets"`
	Disks       map[string]string `json:"disks"`
	Description string            `json:"description"`
}

type QemuStatus struct {
	CPU       float64 `json:"cpu"`
	CPUs      float64 `json:"cpus"`
	Mem       float64 `json:"mem"`
	MaxMem    float64 `json:"maxmem"`
	Disk      float64 `json:"disk"`
	MaxDisk   float64 `json:"maxdisk"`
	DiskWrite float64 `json:"diskwrite"`
	DiskRead  float64 `json:"diskread"`
	NetIn     float64 `json:"netin"`
	NetOut    float64 `json:"netout"`
	Uptime    float64 `json:"uptime"`
	QmpStatus string  `json:"qmpstatus"`
	Status    string  `json:"status"`
	Template  string  `json:"template"`
}

func (qemu QemuVM) Delete() (map[string]interface{}, error) {
	var target string
	var data map[string]interface{}
	var err error

	//fmt.Print("!QemuDelete ", qemu.VMId)

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64)
	data, err = qemu.Node.Proxmox.Delete(target)
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

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/config"
	data, err = qemu.Node.Proxmox.Get(target)
	results = data["data"].(map[string]interface{})
	if err != nil {
		return config, err
	}
	config = QemuConfig{
		Bootdisk:    results["bootdisk"].(string),
		Cores:       results["cores"].(float64),
		Digest:      results["digest"].(string),
		Memory:      results["memory"].(float64),
		Sockets:     results["sockets"].(float64),
		SMBios1:     results["smbios1"].(string),
		Description: results["description"].(string),
	}

	switch results["cores"].(type) {
	default:
		config.Cores = 1
	case float64:
		config.Cores = results["cores"].(float64)
	}
	switch results["sockets"].(type) {
	default:
		config.Sockets = 1
	case float64:
		config.Sockets = results["sockets"].(float64)
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

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/current"
	data, err = qemu.Node.Proxmox.Get(target)
	if err != nil {
		return status, err
	}
	results = data["data"].(map[string]interface{})
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

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/start"
	_, err = qemu.Node.Proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Stop() (string, error) {
	var target string
	var err error

	//fmt.Print("!QemuStop ", qemu.VMId)

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/stop"
	data, err := qemu.Node.Proxmox.Post(target, "")
	if err != nil {
		return "", err
	}

	UPid := data["data"].(string)

	return UPid, nil
}

func (qemu QemuVM) Shutdown() (Task, error) {
	var target string
	var err error

	//fmt.Print("!QemuShutdown ", qemu.VMId)

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/shutdown"
	data, err := qemu.Node.Proxmox.Post(target, "")
	
	if err != err {
		return Task{}, err
	}

	t := Task{
		UPid:    data["data"].(string),
		proxmox: qemu.Node.Proxmox,
	}
	
	return t, err
}

func (qemu QemuVM) Suspend() error {
	var target string
	var err error

	//fmt.Print("!QemuSuspend ", qemu.VMId)

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/suspend"
	_, err = qemu.Node.Proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Resume() error {
	var target string
	var err error

	//fmt.Print("!QemuResume ", qemu.VMId)

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/status/resume"
	_, err = qemu.Node.Proxmox.Post(target, "")
	return err
}

func (qemu QemuVM) Clone(newId float64, name string, targetName string) (string, error) {
	return qemu.CloneToPool(newId, name, targetName, "")
}

func (qemu QemuVM) CloneToPool(newId float64, name string, targetName string, pool string) (string, error) {
	var target string
	var err error

	newVMID := strconv.FormatFloat(newId, 'f', 0, 64)

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/clone"

	form := url.Values{
		"newid":  {newVMID},
		"name":   {name},
		"target": {targetName},
		"full":   {"1"},
	}

	if pool != "" {
		form.Add("pool", pool)
	}

	data, err := qemu.Node.Proxmox.PostForm(target, form)
	if err != err {
		return Task{}, err
	}

	t := Task{
		UPid:    data["data"].(string),
		proxmox: qemu.Node.Proxmox,
	}

	return t, nil
}

func (qemu QemuVM) SetDescription(description string) (error) {
	var target string
	var err error

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/config"

	form := url.Values{
		"description":  {description},
	}

	_, err = qemu.Node.Proxmox.PutForm(target, form)
	if err != err {
		return err
	}

	return nil
}

func (qemu QemuVM) SetMemory(memory string) (error) {
	var target string
	var err error

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/config"

	form := url.Values{
		"memory":  {memory},
	}

	_, err = qemu.Node.Proxmox.PutForm(target, form)
	if err != err {
		return err
	}

	return nil
}

func (qemu QemuVM) SetIPSet(ip string) error {
	var target string
	var err error

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/firewall/options"

	form := url.Values{
		"dhcp":          {"1"},
		"enable":        {"1"},
		"log_level_in":  {"nolog"},
		"log_level_out": {"nolog"},
		"macfilter":     {"1"},
		"ipfilter":      {"1"},
		"ndp":           {"1"},
		"policy_in":     {"ACCEPT"},
		"policy_out":    {"ACCEPT"},
	}

	_, err = qemu.Node.Proxmox.PutForm(target, form)
	if err != nil {
		return err
	}

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/firewall/ipset"

	form = url.Values{
		"name": {"ipfilter-net0"},
	}

	_, err = qemu.Node.Proxmox.PostForm(target, form)
	if err != nil {
		return err
	}

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/firewall/ipset/ipfilter-net0"

	form = url.Values{
		"cidr": {ip},
	}

	_, err = qemu.Node.Proxmox.PostForm(target, form)
	if err != nil {
		return err
	}

	config, err := qemu.Config()
	if err != nil {
		return err
	}

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/config"

	net := ""

	for k, v := range config.Net["net0"] {
		net += k + "=" + v + ","
	}

	form = url.Values{
		"net0": {net + ",firewall=1"},
	}

	_, err = qemu.Node.Proxmox.PutForm(target, form)
	if err != nil {
		return err
	}

	return nil
}

func (qemu QemuVM) ResizeDisk(size string) error {
	var target string
	var err error

	target = "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/resize"

	form := url.Values{
		"disk": {"scsi1"},
		"size": {size + "G"},
	}

	_, err = qemu.Node.Proxmox.PutForm(target, form)
	if err != nil {
		return err
	}

	return nil
}

func (qemu QemuVM) Snapshot(name string, includeRAM bool) (string, error) {
	target := "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/snapshot"

	vmstate := "0"
	if includeRAM == true {
		vmstate = "1"
	}

	form := url.Values{
		"snapname": {name},
		"vmstate":  {vmstate},
	}

	data, err := qemu.Node.Proxmox.PostForm(target, form)
	if err != nil {
		return "", err
	}

	UPid := data["data"].(string)

	return UPid, nil
}

func (qemu QemuVM) Rollback(name string) (string, error) {
	target := "nodes/" + qemu.Node.Node + "/qemu/" + strconv.FormatFloat(qemu.VMId, 'f', 0, 64) + "/snapshot/" + name + "/rollback"

	data, err := qemu.Node.Proxmox.Post(target, "")
	if err != nil {
		return "", err
	}

	UPid := data["data"].(string)

	return UPid, nil
}
