package config

import (
	"train/back/selector"
)

type Config struct {
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	Refresh         int         `json:"refresh"`
	Db              Database    `json:"db"`
	TrainBackendUrl string      `json:"train_backend_url"`
	TrainDataUrl    string      `json:"train_data_url"`
	Dataset         Dataset     `json:"dataset"`
	TrainConfig     TrainConfig `json:"train_config"`
}

func NewDefaultConfig() *Config {
	return &Config{}
}

type Database struct {
	DbName   string            `json:"DbName"`
	Address  string            `json:"Address"`
	Port     int               `json:"Port"`
	Account  string            `json:"Account"`
	Password string            `json:"Password"`
	Selector selector.Selector `json:"selector"`
}

type Dataset struct {
	Input            string `json:"Input"`
	FilePath         string `json:"FilePath"`
	Validation       int    `json:"Validation"`
	CategoriesNumber int    `json:"CategoriesNumber"`
}

type TrainConfig struct {
	Epochs           int     `json:"Epochs"`
	LearningRate     float64 `json:"LearningRate"`
	LossFunction     string  `json:"LossFunction"`
	EarlyStop        bool    `json:"EarlyStop"`
	EarlyStopPatient int     `json:"EarlyStopPatient"`
	ModelSave        string  `json:"ModelSave"`
	LogSave          string  `json:"LogSave"`
	TimeOut          string  `json:"TimeOut"`
}
