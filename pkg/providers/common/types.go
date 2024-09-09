package common

// We're going to use a temporary machine struct to unmarshal from the config file
// and then convert it to the actual machine struct
type RawMachine struct {
	Location   string `yaml:"location"`
	Parameters *struct {
		Count           int    `yaml:"count,omitempty"`
		Type            string `yaml:"type,omitempty"`
		Orchestrator    bool   `yaml:"orchestrator,omitempty"`
		DiskSizeGB      int    `yaml:"disk_size_gb,omitempty"`
		DiskImageURL    string `yaml:"disk_image_url,omitempty"`
		DiskImageFamily string `yaml:"disk_image_family,omitempty"`
	} `yaml:"parameters"`
}
