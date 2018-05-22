package proxmox

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ProxMox struct {
	Hostname                      string
	Username                      string
	password                      string
	VerifySSL                     bool
	BaseURL                       string
	connectionCSRFPreventionToken string
	ConnectionTicket              string
	Client                        *http.Client
}

func NewProxMox(HostName string, UserName string, Password string) (*ProxMox, error) {
	var err error
	var proxmox *ProxMox
	var data map[string]interface{}
	var form url.Values
	var cookies []*http.Cookie
	var tr *http.Transport
	var domain string
	//fmt.Println("!NewProxMox")

	if !strings.HasSuffix(UserName, "@pam") && !strings.HasSuffix(UserName, "@pve") {
		UserName = UserName + "@pam"
	}

	if !strings.HasPrefix(HostName, "http") && !strings.HasPrefix(HostName, "https") {
		HostName = "https://" + HostName
	}

	proxmox = new(ProxMox)
	proxmox.Hostname = HostName
	proxmox.Username = UserName
	proxmox.password = Password
	proxmox.VerifySSL = false
	if len(strings.Split(proxmox.Hostname, ":")) == 1 {
		proxmox.BaseURL = proxmox.Hostname + ":8006"
	}
	proxmox.BaseURL = proxmox.Hostname + "/api2/json/"

	if proxmox.VerifySSL {
		tr = &http.Transport{
			DisableKeepAlives:   false,
			IdleConnTimeout:     0,
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 100}
	} else {
		tr = &http.Transport{
			DisableKeepAlives:   false,
			IdleConnTimeout:     0,
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 100,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		}
	}
	proxmox.Client = &http.Client{
		Transport: tr,
		Timeout:   time.Second * 10}
	form = url.Values{
		"username": {proxmox.Username},
		"password": {proxmox.password},
	}

	data, err = proxmox.PostForm("access/ticket", form)
	if err != nil {
		return nil, err
	} else {
		proxmox.ConnectionTicket = data["ticket"].(string)
    proxmox.connectionCSRFPreventionToken = data["CSRFPreventionToken"].(string)
		proxmox.Client.Jar, err = cookiejar.New(nil)
		domain = proxmox.Hostname

		cookie := &http.Cookie{
			Name:  "PVEAuthCookie",
			Value: data["ticket"].(string),
			Path:  "/",
		}
		cookies = append(cookies, cookie)
		cookieURL, err := url.Parse(domain + "/")
		if err != nil {
			return nil, err
		}
		proxmox.Client.Jar.SetCookies(cookieURL, cookies)

		return proxmox, nil
	}
}

func (proxmox ProxMox) Nodes() (NodeList, error) {
	var err error
	var data map[string]interface{}
	var list NodeList
	var node Node
	var results []interface{}

	//fmt.Println("!Nodes")

	data, err = proxmox.Get("nodes")
	if err != nil {
		return nil, err
	}
	list = make(NodeList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		if val, ok := v["uptime"]; !ok || val == nil {
			fmt.Println("Node probably down. Skipping.")
			continue
		}
		node = Node{
			Mem:      v["mem"].(float64),
			MaxDisk:  v["maxdisk"].(float64),
			Node:     v["node"].(string),
			MaxCPU:   v["maxcpu"].(float64),
			Uptime:   v["uptime"].(float64),
			Id:       v["id"].(string),
			CPU:      v["cpu"].(float64),
			Level:    v["level"].(string),
			NodeType: v["type"].(string),
			Disk:     v["disk"].(float64),
			MaxMem:   v["maxmem"].(float64),
			Proxmox:  proxmox,
		}
		list[node.Node] = node
	}
	return list, nil
}

func (proxmox ProxMox) NextVMId() (string, error) {
	data, err := proxmox.Get("cluster/nextid")
	if err != nil {
		return "", err
	}

	result := data["data"].(string)
	return result, nil
}

func (proxmox ProxMox) DetermineVMPlacement(cpu int64, cores int64, mem int64, overCommitCPU float64, overCommitMem float64) (Node, error) {
	var nodeList NodeList
	var node Node
	var qemuList []QemuVM
	var qemu QemuVM
	var errNode Node
	var usedCPUs int64
	var usedMem int64

	var err error

	nodeList, err = proxmox.Nodes()
	if err != nil {
		return errNode, errors.New("Could not get any nodes.")
	}
	for _, node = range nodeList {
		qemuListSorted, err := node.Qemu()
		if err != nil {
			return errNode, errors.New("Could not get VMs for node " + node.Node + ".")
		}
		//Randomize order of nodes
		qemuList = make([]QemuVM, len(qemuListSorted))
		perm := rand.Perm(len(qemuListSorted))
		i := 0
		for s := range qemuListSorted {
			qemuList[perm[i]] = qemuListSorted[s]
			i++
		}
		for _, qemu = range qemuList {
			usedCPUs = usedCPUs + int64(qemu.CPUs)
			usedMem = usedMem + int64(qemu.MaxMem)
		}
		if (cpu*cores < int64(node.MaxCPU*(1+overCommitCPU))-usedCPUs) && (mem < int64(node.MaxMem*(1+overCommitMem))-usedMem) {
			return node, nil
			//		} else {
			//			fmt.Printf("CPU: %v < %v, Memory: %v < %v\n", cpu*cores, int64(node.MaxCPU*(1+overCommitCPU))-usedCPUs, mem, int64(node.MaxMem*(1+overCommitMem))-usedMem)
		}
	}
	return errNode, errors.New("Not enough free capacity on any of the nodes.")
}

func (proxmox ProxMox) FindVM(VmId string) (QemuVM, error) {
	var nodeList NodeList
	var node Node
	var qemuList QemuList
	var qemu QemuVM
	var errQemu QemuVM
	var ok bool
	var err error

	nodeList, err = proxmox.Nodes()
	if err != nil {
		return errQemu, errors.New("Could not get any nodes.")
	}
	for _, node = range nodeList {
		qemuList, err = node.Qemu()
		if err != nil {
			return errQemu, errors.New("Could not get VMs for node " + node.Node + ".")
		}
		if qemu, ok = qemuList[VmId]; ok {
			return qemu, nil
		}
	}
	return errQemu, errors.New("VM " + VmId + " not found.")
}

func (proxmox ProxMox) Tasks() (TaskList, error) {
	var err error
	var target string
	var data map[string]interface{}
	var list TaskList
	var task Task
	var results []interface{}

	//fmt.Println("!Tasks")
	target = "cluster/tasks"
	data, err = proxmox.Get(target)
	if err != nil {
		return nil, err
	}
	list = make(TaskList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		task = Task{
			UPid:    v["upid"].(string),
			Type:    v["type"].(string),
			ID:      v["id"].(string),
			proxmox: proxmox,
		}
		switch v["status"].(type) {
		default:
			task.Status = ""
		case string:
			task.Status = v["status"].(string)
		}
		switch v["exitstatus"].(type) {
		default:
			task.ExitStatus = ""
		case string:
			task.ExitStatus = v["exitstatus"].(string)
		}
		switch v["pstart"].(type) {
		default:
			task.PStart = 0
		case float64:
			task.PStart = v["pstart"].(float64)
		}
		switch v["starttime"].(type) {
		default:
			task.StartTime = 0
		case float64:
			task.StartTime = v["starttime"].(float64)
		case string:
			s := v["starttime"].(string)
			task.StartTime, err = strconv.ParseFloat(s, 64)
		}
		switch v["endtime"].(type) {
		default:
			task.EndTime = 0
		case float64:
			task.EndTime = v["endtime"].(float64)
		case string:
			s := v["endtime"].(string)
			task.EndTime, err = strconv.ParseFloat(s, 64)
		}
		switch v["pid"].(type) {
		default:
			task.PID = 0
		case float64:
			task.PID = v["pid"].(float64)
		}

		list[task.UPid] = task
	}

	return list, nil
}

func (proxmox ProxMox) Pools() (PoolList, error) {
  var err error
	var target string
	var data map[string]interface{}
	var list PoolList
	var pool Pool
	var results []interface{}

	//fmt.Println("!Tasks")
	target = "pools"
	data, err = proxmox.Get(target)
	if err != nil {
		return nil, err
	}
	list = make(PoolList)
	results = data["data"].([]interface{})
	for _, v0 := range results {
		v := v0.(map[string]interface{})
		pool = Pool{
			Poolid:    v["poolid"].(string),
			proxmox: proxmox,
		}

		list[pool.Poolid] = pool
  }

  return list, nil
}

func (proxmox ProxMox) NewPool(name string, comment string) (map[string]interface{}, error) {
  poolForm := url.Values{}
  poolForm.Set("poolid", name)
  poolForm.Set("comment", comment)

  result, err := proxmox.PostForm("pools", poolForm)
  if err != nil {
    fmt.Println("Error while posting form")
    fmt.Println(err)
    return result, err
  }

  fmt.Printf("Result: %s", result)

  return result, nil
}

func (proxmox ProxMox) UpdatePool(name string, comment string) (map[string]interface{}, error) {
  poolForm := url.Values{}
  poolForm.Set("poolid", name)
  poolForm.Set("comment", comment)

  result, err := proxmox.PutForm("pools/" + name, poolForm)
  if err != nil {
    fmt.Println("Error while posting form")
    fmt.Println(err)
    return result, err
  }

  fmt.Printf("Result: %s", result)

  return result, nil
}

func (proxmox ProxMox) DeletePool(name string) error {
  result, err := proxmox.Delete(fmt.Sprintf("pools/%s", name))

  if err != nil {
    fmt.Printf("Error deleting pool: %s", name)
    fmt.Println(err)
    fmt.Printf("Result was: %s", result)
  }

  fmt.Printf("Created pool: %s\n", name)

  return nil
}

func (proxmox ProxMox) PostForm(endpoint string, form url.Values) (map[string]interface{}, error) {
	var target string
	var data interface{}
	var req *http.Request

	//fmt.Println("!PostForm")

	target = proxmox.BaseURL + endpoint
	//target = "http://requestb.in/1ls8s9d1"
	//fmt.Println("POST form " + target)

	req, err := http.NewRequest("POST", target, bytes.NewBufferString(form.Encode()))

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))
	if proxmox.connectionCSRFPreventionToken != "" {
		req.Header.Add("CSRFPreventionToken", proxmox.connectionCSRFPreventionToken)
	}

  fmt.Printf("Posting form values: %s\n", req)

	r, err := proxmox.Client.Do(req)
	if err != nil {
		fmt.Println("Error while posting")
		fmt.Println(err)
		return nil, err
	}
	//fmt.Print("HTTP status ")
	//fmt.Println(r.StatusCode)
	if r.StatusCode != 200 {
		return nil, errors.New("HTTP Error " + r.Status)
		//	} else {
	}

	response, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		fmt.Println("Error while reading body")
		fmt.Println(err)
		return nil, err
	}
	err = json.Unmarshal(response, &data)
	if err != nil {
		fmt.Println("Error while processing JSON")
		fmt.Println(err)
		return nil, err
	}
	m := data.(map[string]interface{})
	switch m["data"].(type) {
	case map[string]interface{}:
		d := m["data"].(map[string]interface{})
		return d, nil
	}
	return m, nil
}

func (proxmox ProxMox) Post(endpoint string, input string) (map[string]interface{}, error) {
	var target string
	var data interface{}
	var req *http.Request

	//fmt.Println("!Post")

	target = proxmox.BaseURL + endpoint
	//target = "http://requestb.in/1ls8s9d1"
	//fmt.Println("POST form " + target)

	req, err := http.NewRequest("POST", target, bytes.NewBufferString(input))

	req.Header.Add("Content-Length", strconv.Itoa(len(input)))
	if proxmox.connectionCSRFPreventionToken != "" {
		req.Header.Add("CSRFPreventionToken", proxmox.connectionCSRFPreventionToken)
	}
	r, err := proxmox.Client.Do(req)
	if err != nil {
		fmt.Println("Error while posting")
		fmt.Println(err)
		return nil, err
	}
	//fmt.Print("HTTP status ")
	//fmt.Println(r.StatusCode)
	if r.StatusCode != 200 {
		return nil, errors.New("HTTP Error " + r.Status)
		//	} else {
	}

	response, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		fmt.Println("Error while reading body")
		fmt.Println(err)
		return nil, err
	}
	err = json.Unmarshal(response, &data)
	if err != nil {
		fmt.Println("Error while processing JSON")
		fmt.Println(err)
		return nil, err
	}
	m := data.(map[string]interface{})
	switch m["data"].(type) {
	case map[string]interface{}:
		d := m["data"].(map[string]interface{})
		return d, nil
	}
	return m, nil
}

func (proxmox ProxMox) PutForm(endpoint string, form url.Values) (map[string]interface{}, error) {
	var target string
	var data interface{}
	var req *http.Request

	//fmt.Println("!PutForm")

	target = proxmox.BaseURL + endpoint

	req, err := http.NewRequest("PUT", target, bytes.NewBufferString(form.Encode()))

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(form.Encode())))
	if proxmox.connectionCSRFPreventionToken != "" {
		req.Header.Add("CSRFPreventionToken", proxmox.connectionCSRFPreventionToken)
	}
	r, err := proxmox.Client.Do(req)
	defer r.Body.Close()
	if err != nil {
		fmt.Println("Error while puting")
		fmt.Println(err)
		return nil, err
	}
	//fmt.Print("HTTP status ")
	//fmt.Println(r.StatusCode)
	if r.StatusCode != 200 {
		//spew.Dump(r)
		return nil, errors.New("HTTP Error " + r.Status)
		//	} else {
		//		spew.Dump(r)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("Error while reading body")
		fmt.Println(err)
		return nil, err
	}
	err = json.Unmarshal(response, &data)
	if err != nil {
		fmt.Println("Error while processing JSON")
		fmt.Println(err)
		return nil, err
	}
	m := data.(map[string]interface{})
	//spew.Dump(m)
	switch m["data"].(type) {
	case map[string]interface{}:
		d := m["data"].(map[string]interface{})
		return d, nil
	}
	return m, nil
}

func (proxmox ProxMox) GetRaw(endpoint string) ([]byte, error) {
	target := proxmox.BaseURL + endpoint
	r, err := proxmox.Client.Get(target)
	if err != nil {
		return nil, err
	}
	response, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (proxmox ProxMox) Get(endpoint string) (map[string]interface{}, error) {
	var target string
	var data interface{}

	//fmt.Println("!get")

	target = proxmox.BaseURL + endpoint
	//target = "http://requestb.in/1ls8s9d1"
	//fmt.Println("GET " + target)
	r, err := proxmox.Client.Get(target)
	if err != nil {
		return nil, err
	}
	response, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(response, &data)
	if err != nil {
		return nil, err
	}
	m := data.(map[string]interface{})
	//d := m["data"].(map[string]interface{})
	return m, nil
}

func (proxmox ProxMox) GetBytes(endpoint string) ([]byte, error) {
	var target string

	//fmt.Println("!getBytes")

	target = proxmox.BaseURL + endpoint
	//target = "http://requestb.in/1ls8s9d1"
	//fmt.Println("GET " + target)
	r, err := proxmox.Client.Get(target)
	if err != nil {
		return nil, err
	}
	response, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (proxmox ProxMox) Delete(endpoint string) (map[string]interface{}, error) {
	var target string
	var data interface{}
	var req *http.Request

	//fmt.Println("!PostForm")

	target = proxmox.BaseURL + endpoint
	//target = "http://requestb.in/1ls8s9d1"
	//fmt.Println("DELETE " + target)

	req, err := http.NewRequest("DELETE", target, nil)

	req.Header.Add("CSRFPreventionToken", proxmox.connectionCSRFPreventionToken)

	r, err := proxmox.Client.Do(req)
	if err != nil {
		fmt.Println("Error while deleting")
		fmt.Println(err)
		return nil, err
	}
	//fmt.Print("HTTP status ")
	//fmt.Println(r.StatusCode)
	if r.StatusCode != 200 {
		return nil, errors.New("HTTP Error " + r.Status)
		//	} else {
	}

	response, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		fmt.Println("Error while reading body")
		fmt.Println(err)
		return nil, err
	}
	err = json.Unmarshal(response, &data)
	if err != nil {
		fmt.Println("Error while processing JSON")
		fmt.Println(err)
		return nil, err
	}
	m := data.(map[string]interface{})
	switch m["data"].(type) {
	case map[string]interface{}:
		d := m["data"].(map[string]interface{})
		return d, nil
	}
	return m, nil
}
