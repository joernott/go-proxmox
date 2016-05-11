package proxmox

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"

	_ "github.com/bndr/gopencils"
	_ "github.com/davecgh/go-spew/spew"
)

type ProxMox struct {
	Hostname                      string
	Username                      string
	password                      string
	VerifySSL                     bool
	BaseURL                       string
	connectionCSRFPreventionToken string
	ConnectionTicket              string
	client                        *http.Client
}

func NewProxMox(HostName string, UserName string, Password string) (*ProxMox, error) {
	var err error
	var proxmox *ProxMox
	var data map[string]interface{}
	var form url.Values
	var cookies []*http.Cookie
	var testcookies []*http.Cookie
	var tr *http.Transport
	var domain string
	//fmt.Println("!NewProxMox")

	proxmox = new(ProxMox)
	proxmox.Hostname = HostName
	proxmox.Username = UserName
	proxmox.password = Password
	proxmox.VerifySSL = false
	proxmox.BaseURL = "https://" + proxmox.Hostname + ":8006/api2/json/"

	if proxmox.VerifySSL {
		tr = &http.Transport{}
	} else {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	proxmox.client = &http.Client{Transport: tr}
	form = url.Values{
		"username": {proxmox.Username + "@pam"},
		"password": {proxmox.password},
	}

	data, err = proxmox.PostForm("access/ticket", form)
	if err != nil {
		return nil, err
	} else {
		proxmox.connectionCSRFPreventionToken = data["CSRFPreventionToken"].(string)
		proxmox.ConnectionTicket = data["ticket"].(string)
		proxmox.client.Jar, err = cookiejar.New(nil)
		domain = proxmox.Hostname
		cookie := &http.Cookie{
			Name:   "PVEAuthCookie",
			Value:  data["ticket"].(string),
			Path:   "/",
			Domain: domain,
		}
		cookies = append(cookies, cookie)
		cookie = &http.Cookie{
			Name:   "CSRFPreventionToken",
			Value:  data["CSRFPreventionToken"].(string),
			Path:   "/",
			Domain: domain,
		}
		cookies = append(cookies, cookie)
		cookieURL, err := url.Parse("https://" + domain + "/")
		if err != nil {
			return nil, err
		}
		proxmox.client.Jar.SetCookies(cookieURL, cookies)

		domain = "requestb.in"
		cookie = &http.Cookie{
			Name:   "PVEAuthCookie",
			Value:  data["ticket"].(string),
			Path:   "/",
			Domain: domain,
		}
		testcookies = append(testcookies, cookie)
		cookie = &http.Cookie{
			Name:   "CSRFPreventionToken",
			Value:  data["CSRFPreventionToken"].(string),
			Path:   "/",
			Domain: domain,
		}
		testcookies = append(testcookies, cookie)
		cookieURL, err = url.Parse("https://" + domain + "/")
		if err != nil {
			return nil, err
		}
		proxmox.client.Jar.SetCookies(cookieURL, testcookies)

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
			proxmox:  proxmox,
		}
		list[node.Node] = node
	}
	return list, nil
}

func (proxmox ProxMox) maxVMId() (float64, error) {
	var id float64
	var maxId float64
	var err error
	var nodes NodeList
	var node Node

	//fmt.Println("!maxVMId")
	maxId = 0

	nodes, err = proxmox.Nodes()
	if err != nil {
		return 0, err
	}
	for _, node = range nodes {
		id, err = node.MaxQemuId()
		if err != nil {
			return 0, err
		}
		if id > maxId {
			maxId = id
		}
	}
	return maxId, err
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
	r, err := proxmox.client.Do(req)
	defer r.Body.Close()
	if err != nil {
		fmt.Println("Error while posting")
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

func (proxmox ProxMox) Get(endpoint string) (map[string]interface{}, error) {
	var target string
	var data interface{}

	//fmt.Println("!get")

	target = proxmox.BaseURL + endpoint
	//target = "http://requestb.in/1ls8s9d1"
	//fmt.Println("GET " + target)
	r, err := proxmox.client.Get(target)
	defer r.Body.Close()
	if err != nil {
		return nil, err
	}
	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(response, &data)
	if err != nil {
		return nil, err
	}
	m := data.(map[string]interface{})
	//d := m["data"].(map[string]interface{})
	//spew.Dump(m)
	return m, nil
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

	r, err := proxmox.client.Do(req)
	defer r.Body.Close()
	if err != nil {
		fmt.Println("Error while deleting")
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
