package main

import (
	"flag"
	"fmt"

	"github.com/minicago/gooj/cmd"
	"github.com/minicago/gooj/config"
	"github.com/minicago/gooj/server"
)

func main() {
	// fmt.Println("Hello, World!")

	var method string
	var background bool
	var configPath string
	flag.StringVar(&method, "method", "None", "run | cmd")
	flag.BoolVar(&background, "background", false, "--background = true | false")
	flag.StringVar(&configPath, "config", "config/config.yaml", "path to config file")
	flag.Parse()

	// Load configuration
	if err := config.Load(configPath); err != nil {
		fmt.Printf("Warning: failed to load config: %v, using defaults\n", err)
	} else {
		fmt.Printf("Configuration loaded from %s\n", configPath)
	}

	// // Initialize the SQLite database
	// if err := sql_service.Init("data/app.db"); err != nil {
	// 	panic("Failed to initialize database: " + err.Error())
	// }

	switch method {
	case "run":
		// start file service and judge goroutine before starting server
		// initialize sqlite DB (data/app.db)

		server.StartServer(background)
	case "cmd":
		cmd.StartCmdConsole()
	}
}
