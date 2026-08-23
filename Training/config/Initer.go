package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"train/back/selector"
	"train/pkg"
)

type Initer interface {
	Init(file string, config *Config) error
}

type DefaultIniter struct{}

func NewDefaultIniter() *DefaultIniter {
	return &DefaultIniter{}
}

func (i *DefaultIniter) Init(file string, config *Config) error {
	if file == "" {
		var projectName string
		var projectDescription string
		var projectRefresh int
		var trainBackendUrl string
		var dbName string
		var address string
		var port int
		var account string
		var password string
		var input string
		var filePath string
		var validation int
		var categoriesNumber int
		var epochs int
		var learningRate float64
		var lossFunction string
		var earlyStop bool
		var earlyStopPatient int
		var modelSave string
		var logSave string
		var selectorfile string
		flag.StringVar(&projectDescription, "Description", "", "Description of the plan")
		flag.IntVar(&projectRefresh, "Refresh", 1, "Refresh interval for training status")
		flag.StringVar(&trainBackendUrl, "BackendUrl", "", "Training URL")
		flag.StringVar(&dbName, "DbName", "", "Types of databases")
		flag.StringVar(&address, "Address", "", "Address of database")
		flag.IntVar(&port, "Port", 0, "Port of database")
		flag.StringVar(&account, "Account", "", "Account of database")
		flag.StringVar(&password, "Password", "", "Password of database")
		flag.StringVar(&input, "Input", "", "Data type")
		flag.IntVar(&validation, "Validation", 30, "The data ratio used for testing")
		flag.IntVar(&categoriesNumber, "CategoriesNumber", 2, "Categories of data")
		flag.IntVar(&epochs, "Epochs", 1, "Number of epochs")
		flag.Float64Var(&learningRate, "LearningRate", 0.0, "Learning rate")
		flag.StringVar(&lossFunction, "LossFunction", "", "Loss function")
		flag.BoolVar(&earlyStop, "EarlyStop", false, "Whether to enable early stopping")
		flag.IntVar(&earlyStopPatient, "EarlyStopPatient", 5, "Number of patience rounds in the early stopping")
		flag.StringVar(&modelSave, "ModelSave", "", "Model storage path")
		flag.StringVar(&logSave, "LogSave", "", "Log storage path")
		flag.StringVar(&filePath, "FilePath", "", "Data storage path")
		flag.StringVar(&projectName, "Name", "01", "Name of the plan to be executed")
		flag.StringVar(&selectorfile, "SelectorFile", "", "File path for the selector configuration")
		flag.Parse()
		if trainBackendUrl == "" || dbName == "" || address == "" || account == "" || password == "" || input == "" || dbName == "machine" && filePath == "" || selectorfile == "" {
			return errors.New("the required fields is missing")
		}
		config.Name = projectName
		config.Description = projectDescription
		config.Refresh = projectRefresh
		config.TrainBackendUrl = trainBackendUrl
		config.Db.DbName = dbName
		config.Db.Address = address
		config.Db.Password = password
		config.Db.Account = account
		config.Db.Port = port
		config.Dataset.Input = input
		config.Dataset.FilePath = filePath
		config.Dataset.Validation = validation
		config.Dataset.CategoriesNumber = categoriesNumber
		config.TrainConfig.EarlyStop = earlyStop
		config.TrainConfig.EarlyStopPatient = earlyStopPatient
		config.TrainConfig.ModelSave = modelSave
		config.TrainConfig.LogSave = logSave
		config.TrainConfig.Epochs = epochs
		config.TrainConfig.LearningRate = learningRate
		config.TrainConfig.LossFunction = lossFunction
		criterias, err := pkg.Analysis(selectorfile)
		if err != nil {
			return err
		}
		defaultselector := selector.NewDefaultSelector(criterias...)
		config.Db.Selector = defaultselector
	} else {
		configfile, err := os.Open(file)
		defer configfile.Close()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(configfile)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, config)
		if err != nil {
			return err
		}
	}
	return nil
}
