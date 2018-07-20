package proxmox

type Pool struct {
	Poolid  string `json:"poolid"`
	proxmox ProxMox
}

type PoolList map[string]Pool

type PoolResult struct {
	Data Task `json:"data"`
}
