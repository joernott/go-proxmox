package proxmox

type Task struct {
	UPid      string
	Type      string
	Status    string
	Node      Node
	PID       float64
	PStart    float64
	StartTime float64
	EndTime   float64
	ID        string
}

type TaskList map[string]Task
