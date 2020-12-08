package main

import (
	"bjut_eth/eth"
	"bjut_eth/node"
	"bjut_eth/p2p"
	"bjut_eth/rpc"
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"unicode"

	"github.com/naoina/toml"
)

type SampleService struct{}

func (s *SampleService) Protocols() []p2p.Protocol { return nil }
func (s *SampleService) APIs() []rpc.API {
	return []rpc.API{{
		Namespace: "sample",
		Version:   "1.0",
		Service:   new(SampleAPI),
		Public:    true,
	}}
}
func (s *SampleService) Start(*p2p.Server) error { fmt.Println("Service starting..."); return nil }
func (s *SampleService) Stop() error             { fmt.Println("Service stopping..."); return nil }

func NewSampleService(*node.ServiceContext) (node.Service, error) { return NewSampleAPI(), nil }

type SampleAPI struct {
	*SampleService
}

func NewSampleAPI() *SampleAPI {
	return &SampleAPI{new(SampleService)}
}

func (s *SampleAPI) Echo() string {
	fmt.Println("rpc called sample")
	return "SampleAPI"
}

type gethConfig struct {
	Eth  eth.Config
	Node node.Config
}

func loadConfig(file string, cfg *gethConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}
