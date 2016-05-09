package proxmox

type Volume struct {
	Size    float64
	VolId   string
	VmId    string
	Format  string
	Content string
	Used    float64
	storage Storage
}

type VolumeList map[string]Volume
