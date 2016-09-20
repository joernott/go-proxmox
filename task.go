package proxmox

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	_ "github.com/davecgh/go-spew/spew"
)

type Task struct {
	UPid       string  `json:"upid"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	ExitStatus string  `json:"exitstatus"`
	PID        float64 `json:"pid"`
	PStart     float64 `json:"pstart"`
	StartTime  float64 `json:"starttime"`
	EndTime    float64 `json:"endtime"`
	ID         string  `json:"id"`
	proxmox    ProxMox
}

type TaskList map[string]Task

type TaskResult struct {
	Data Task `json:"data"`
}

func (task Task) GetStatus() (string, string, error) {
	var target string
	var err error
	var raw []byte
	var data TaskResult

	upidParts := strings.Split(task.UPid, ":")
	target = "nodes/" + upidParts[1] + "/tasks/" + task.UPid + "/status"
	//fmt.Println("target  " + target)
	raw, err = task.proxmox.GetBytes(target)
	if err != nil {
		return "", "", err
	}
	err = json.Unmarshal(raw, &data)
	if err != nil {
		return "", "", err
	}
	return data.Data.Status, data.Data.ExitStatus, nil
}

func (task Task) WaitForStatus(status string, timeout int) (string, error) {
	var i int
	var err error
	var actstatus string
	var exitstatus string
	for i = 0; i < timeout; i++ {
		actstatus, exitstatus, err = task.GetStatus()
		if err != nil {
			return "", err
		}
		if actstatus == status {
			return exitstatus, nil
		}
		time.Sleep(time.Second * 1)
	}
	return "", errors.New("Timeout reached")
}
