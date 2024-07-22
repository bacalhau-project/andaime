package utils

type Config struct {
	AWS struct {
		Regions []string `yaml:"regions"`
	} `yaml:"aws"`
}
