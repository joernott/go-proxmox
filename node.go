package proxmox

import (
	"fmt"
	"net/url"
	"strconv"

	_ "github.com/davecgh/go-spew/spew"
)

type Node struct {
	Mem      float64 `json:"mem"`
	MaxDisk  float64 `json:"maxdisk"`
	Node     string  `json:"node"`
	MaxCPU   float64 `json:"maxcpu"`
	Uptime   float64 `json:"uptime"`
	Id       string  `json:"id"`
	CPU      float64 `json:"cpu"`
	Level    string  `json:"level"`
	NodeType string  `json:"nodetype"`
	Disk     float64 `json:"disk"`
	MaxMem   float64 `json:"maxmem"`
	Proxmox  ProxMox
}

type NodeList map[string]Node

type NodeListResult struct {
	Data NodeList `json:"data"`
}

func (node Node) Qemu() (QemuList, error) {
	var err error
	var data map[string]interface{}
	var list QemuList
	var vm QemuVM
	var results []interface{}

	//fmt.Println("!Qemu")

	data, err = node.Proxmox.Get("nodes/" + node.Node + "/qemu")
	if err != nil {
		return nil, err
	}

	list = make(QemuList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		vm = QemuVM{
			Mem:    v["mem"].(float64),
			CPUs:   v["cpus"].(float64),
			NetOut: v["netout"].(float64),
			//			PID:       v["pid"].(string),
			Disk:      v["disk"].(float64),
			MaxMem:    v["maxmem"].(float64),
			Status:    v["status"].(string),
			Template:  v["template"].(string),
			NetIn:     v["netin"].(float64),
			MaxDisk:   v["maxdisk"].(float64),
			Name:      v["name"].(string),
			DiskWrite: v["diskwrite"].(float64),
			CPU:       v["cpu"].(float64),
			VMId:      v["vmid"].(float64),
			DiskRead:  v["diskread"].(float64),
			Uptime:    v["uptime"].(float64),
			Node:      node,
		}
		list[strconv.FormatFloat(vm.VMId, 'f', 0, 64)] = vm
	}

	return list, nil
}

func (node Node) MaxQemuId() (float64, error) {
	var list QemuList
	var vm QemuVM
	var id float64
	var err error

	//fmt.Println("!MaxQemuId")

	id = 0
	list, err = node.Qemu()
	if err != nil {
		return 0, err
	}

	for _, vm = range list {
		if vm.VMId > id {
			id = vm.VMId
		}
	}
	return id, nil
}

func (node Node) Storages() (StorageList, error) {
	var err error
	var data map[string]interface{}
	var list StorageList
	var storage Storage
	var results []interface{}

	//fmt.Println("!Storages")

	data, err = node.Proxmox.Get("nodes/" + node.Node + "/storage")
	if err != nil {
		return nil, err
	}
	//spew.Dump(data)
	list = make(StorageList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		storage = Storage{
			StorageType: v["type"].(string),
			Active:      v["active"].(float64),
			Total:       v["total"].(float64),
			Content:     v["content"].(string),
			Shared:      v["shared"].(float64),
			Storage:     v["storage"].(string),
			Used:        v["used"].(float64),
			Avail:       v["avail"].(float64),
			Node:        node,
		}
		list[storage.Storage] = storage
	}

	return list, nil
}

func (node Node) CreateQemuVM(Name string, Sockets int, Cores int, MemorySize int, DiskSize string) (string, error) {
	var err error
	var newVmId string
	var storageList StorageList
	//var storage Storage
	var results map[string]interface{}
	var storageId string
	var form url.Values
	var target string

	//fmt.Println("!CreateQemuVM")

	i, err := node.Proxmox.maxVMId()
	if err != nil {
		return "", err
	}
	newVmId = strconv.FormatFloat(i+1, 'f', 0, 64)
	//fmt.Println("new VM ID: " + newVmId)
	storageList, err = node.Storages()
	results, err = storageList["local"].CreateVolume("vm-"+newVmId+"-disk-0.qcow2", DiskSize, newVmId)
	if err != nil {
		return "", err
	}
	storageId = results["data"].(string)

	//fmt.Println("!CreateVolume")

	form = url.Values{
		"vmid":    {newVmId},
		"memory":  {strconv.Itoa(MemorySize)},
		"sockets": {strconv.Itoa(Sockets)},
		"cores":   {strconv.Itoa(Cores)},
		"net0":    {"virtio,bridge=vmbr0"},
		"virtio0": {storageId},
	}
	if Name != "" {
		form.Set("name", Name)
	}

	target = "nodes/" + node.Node + "/qemu"
	results, err = node.Proxmox.PostForm(target, form)
	if err != nil {
		fmt.Println("Error creating VM!!!")
		return "", err
	}
	//fmt.Println("VM " + newVmId + " created")

	//spew.Dump(results)
	return newVmId, err
}

func (node Node) VZDump(VmId string, BWLimit int, Compress string, IONice int, LockWait int, Mode string) (string, error) {
	var form url.Values
	var target string
	var err error
	var results map[string]interface{}

	form = url.Values{
		"vmid":     {VmId},
		"compress": {Compress},
		"lockwait": {strconv.Itoa(LockWait)},
		"mode":     {Mode},
	}
	if BWLimit > 0 {
		form.Set("bwlimit", strconv.Itoa(BWLimit))
	}
	if IONice > 0 {
		form.Set("ionice", strconv.Itoa(IONice))
	}
	target = "nodes/" + node.Node + "/vzdump"
	results, err = node.Proxmox.PostForm(target, form)
	//spew.Dump(results)
	if err != nil {
		fmt.Println("Error dumping VM!")
		return "", err
	}
	return results["data"].(string), nil
}

func (node Node) Tasks(Limit int, Start int, UserFilter string, VmId string) (TaskList, error) {
	var err error
	var target string
	var data map[string]interface{}
	var list TaskList
	var task Task
	var results []interface{}

	//fmt.Println("!Tasks")
	target = "nodes/" + node.Node + "/tasks?"
	if Limit > 0 {
		target = target + "limit=" + strconv.Itoa(Limit) + "&"
	}
	if Start > 0 {
		target = target + "start=" + strconv.Itoa(Start) + "&"
	}
	if UserFilter != "" {
		target = target + "userfilter=" + UserFilter + "&"
	}
	if VmId != "" {
		target = target + "vmid=" + VmId + "&"
	}
	target = target[0 : len(target)-1]
	data, err = node.Proxmox.Get(target)
	if err != nil {
		return nil, err
	}
	list = make(TaskList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		task = Task{
			UPid:      v["upid"].(string),
			Type:      v["type"].(string),
			Status:    v["status"].(string),
			PID:       v["pid"].(float64),
			PStart:    v["pstart"].(float64),
			StartTime: v["starttime"].(float64),
			EndTime:   v["endtime"].(float64),
			ID:        v["id"].(string),
			proxmox:   node.Proxmox,
		}
		list[task.UPid] = task
	}

	return list, nil
}
