package proxmox

import (
	"fmt"
	"net/url"

	_ "github.com/davecgh/go-spew/spew"
)

type Storage struct {
	StorageType string  `json:"type"`
	Active      float64 `json:"active"`
	Total       float64 `json:"total"`
	Content     string  `json:"content"`
	Shared      float64 `json:"shared"`
	Storage     string  `json:"storage"`
	Used        float64 `json:"used"`
	Avail       float64 `json:"avail"`
	Node        Node
}

type StorageList map[string]Storage

func (storage Storage) CreateVolume(FileName string, DiskSize string, VmId string) (map[string]interface{}, error) {
	var form url.Values
	var err error
	var data map[string]interface{}
	var target string

	//fmt.Println("!CreateVolume")

	form = url.Values{
		"filename": {FileName},
		//		"node":     {storage.node.Node},
		"size":   {DiskSize},
		"format": {"qcow2"},
		"vmid":   {VmId},
	}

	target = "nodes/" + storage.Node.Node + "/storage/" + storage.Storage + "/content"
	data, err = storage.Node.Proxmox.PostForm(target, form)
	if err != nil {
		fmt.Println("Error!!!")
		return nil, err
	}
	//fmt.Println("Storage created")
	return data, err
}

func (storage Storage) Volumes() (VolumeList, error) {
	var err error
	var target string
	var data map[string]interface{}
	var list VolumeList
	var volume Volume
	var results []interface{}

	//fmt.Println("!Volumes")

	target = "nodes/" + storage.Node.Node + "/storage/" + storage.Storage + "/content"
	data, err = storage.Node.Proxmox.Get(target)
	if err != nil {
		return nil, err
	}

	list = make(VolumeList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		volume = Volume{
			Size:    v["size"].(float64),
			VolId:   v["volid"].(string),
			VmId:    v["vmid"].(string),
			Format:  v["format"].(string),
			Content: v["content"].(string),
			Used:    v["used"].(float64),
			storage: storage,
		}
		list[volume.VolId] = volume
	}
	return list, nil
}
