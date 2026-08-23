package config

type Config struct {
	DbName   string `json:"DbName"`
	Account  string `json:"Account"`
	Password string `json:"Password"`
	URL      string `json:"Url"`
}
