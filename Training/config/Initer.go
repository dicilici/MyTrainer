package config

import (
	"encoding/json"
	"io"
	"os"
)

type Initer interface {
	Init(file string, config *Config) error
}

type DefaultIniter struct{}

func NewDefaultIniter() *DefaultIniter {
	return &DefaultIniter{}
}

func (i *DefaultIniter) Init(file string, config *Config) error {
	configfile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer configfile.Close()
	data, err := io.ReadAll(configfile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, config)
	if err != nil {
		return err
	}
	return nil
}
