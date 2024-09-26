package common

type RawMachineParams struct {
	Count           int    `yaml:"count,omitempty"`
	Type            string `yaml:"type,omitempty"`
	Orchestrator    bool   `yaml:"orchestrator,omitempty"`
	DiskSizeGB      int    `yaml:"disk_size_gb,omitempty"`
	DiskImageURL    string `yaml:"disk_image_url,omitempty"`
	DiskImageFamily string `yaml:"disk_image_family,omitempty"`
}

type RawMachine struct {
	Location   string           `yaml:"location"`
	Parameters RawMachineParams `yaml:"parameters,omitempty"`
}
