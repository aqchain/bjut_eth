package main

import (
	"bjut_eth/rpc"
	"fmt"
	"gopkg.in/urfave/cli.v1"
	"net"
	"os"
)

func main() {

	app := cli.NewApp()
	app.Usage = "rpc测试"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "api",
			Value:  "http://localhost:8888",
			Usage:  "simulation API URL",
			EnvVar: "P2PSIM_API_URL",
		},
	}
	app.Before = func(ctx *cli.Context) error {
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:   "newHttpServer",
			Usage:  "new server",
			Action: newServer,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "address",
					Value: "127.0.0.1:8545",
					Usage: "http address",
				},
			},
		},
	}

	app.Run(os.Args)

	//client := rpc.DialInProc(server)
	//defer client.Close()
	//
	//var resp Result
	//if err := client.Call(&resp, "test_test", "hello", 10, &Args{"world"}); err != nil {
	//	log.Error("test error",err.Error())
	//}

}

func newServer(ctx *cli.Context) error {

	address := ctx.String("address")

	server := rpc.NewServer()
	test := new(Test)
	err := server.RegisterName("test", test)
	if err != nil {
		fmt.Println("test error", err.Error())
	}
	defer server.Stop()
	var listener net.Listener
	if listener, err = net.Listen("tcp", address); err != nil {
		fmt.Println("test error", err.Error())
	}
	cors := []string{"*"}
	vhost := []string{""}
	go rpc.NewHTTPServer(cors, vhost, server).Serve(listener)

	//client, err := rpc.Dial("http://127.0.0.1:8545")
	//if err != nil {
	//	fmt.Println("test error", err.Error())
	//}
	//
	//var resp Result
	//if err := client.Call(&resp, "test_echo", "hello", 10, &Args{"world"}); err != nil {
	//	fmt.Println("test error",err.Error())
	//}
	//
	//spew.Dump(resp)
	select {}
}
