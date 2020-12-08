package main

import (
	"bjut_eth/eth"
	"bjut_eth/node"
	"bjut_eth/p2p"
	"bjut_eth/rpc"
	"os"
	"path/filepath"

	"bjut_eth/crypto"
	"bjut_eth/log"
	"gopkg.in/urfave/cli.v1"
)

var testLog log.Logger

func init() {
	testLog = log.New("module", "node_test")
	testLog.SetHandler(log.LvlFilterHandler(log.LvlTrace, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
}

func main() {
	app := cli.NewApp()
	app.Usage = "node test"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "dataDir",
			Value: "",
			Usage: "data dir",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "test",
			Usage: "test Command",
			Action: func(c *cli.Context) error {
				testLog.Info("test Command")
				return nil
			},
		},
		{
			Name:   "genKey",
			Usage:  "generate node key",
			Action: generateKey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "dir",
					Value: "",
					Usage: "data dir",
				},
			},
		},
		{
			Name:   "genesis",
			Usage:  "generate genesis block",
			Action: genesis,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config",
					Value: "",
					Usage: "genesis.json file path",
				},
			},
		},
	}
	app.Action = newNode
	app.Run(os.Args)
}

func genesis() {

}

func generateKey(ctx *cli.Context) error {

	dir := ctx.String("dir")

	file := filepath.Join(dir, "node.key")
	nodeKey, err := crypto.GenerateKey()
	if err != nil {
		testLog.Error(err.Error())
		return err
	}
	err = crypto.SaveECDSA(file, nodeKey)
	if err != nil {
		testLog.Error(err.Error())
		return err
	}
	testLog.Info("generate key in " + file)
	return nil
}

func newNode(ctx *cli.Context) {

	dataDir := ctx.String("dataDir")

	cfg := &gethConfig{}
	err := loadConfig(filepath.Join(dataDir, "config.toml"), cfg)
	if err != nil {
		testLog.Error(err.Error())
		return
	}

	nodeConfig := cfg.Node
	nodeConfig.Logger = testLog
	nodeConfig.HTTPModules = append(nodeConfig.HTTPModules, "sample", "admin", "eth", "personal")

	stack, err := node.New(&nodeConfig)
	if err != nil {
		testLog.Error(err.Error())
	}

	err = stack.Register(NewNoopService)
	if err != nil {
		testLog.Error(err.Error())
	}

	err = stack.Register(NewSampleService)
	if err != nil {
		testLog.Error(err.Error())
	}

	ethConfig := eth.DefaultConfig
	err = stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		fullNode, err := eth.New(ctx, &ethConfig)
		return fullNode, err
	})

	err = stack.Start()
	if err != nil {
		testLog.Error(err.Error())
	}

	stack.Wait()
}

// NoopService is a trivial implementation of the Service interface.
type NoopService struct{}

func (s *NoopService) Protocols() []p2p.Protocol { return nil }
func (s *NoopService) APIs() []rpc.API           { return nil }
func (s *NoopService) Start(*p2p.Server) error   { return nil }
func (s *NoopService) Stop() error               { return nil }

func NewNoopService(*node.ServiceContext) (node.Service, error) { return new(NoopService), nil }

// Set of services all wrapping the base NoopService resulting in the same method
// signatures but different outer types.
type NoopServiceA struct{ NoopService }
type NoopServiceB struct{ NoopService }
type NoopServiceC struct{ NoopService }

func NewNoopServiceA(*node.ServiceContext) (node.Service, error) { return new(NoopServiceA), nil }
func NewNoopServiceB(*node.ServiceContext) (node.Service, error) { return new(NoopServiceB), nil }
func NewNoopServiceC(*node.ServiceContext) (node.Service, error) { return new(NoopServiceC), nil }
