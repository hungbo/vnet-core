package main

import (
	"embed"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--watch" {
		runWatch()
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "VNET Client",
		Width:     1024,
		Height:    768,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.Startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func runWatch() {
	if len(os.Args) < 5 {
		return
	}
	pid, _ := strconv.Atoi(os.Args[2])
	machineCode := os.Args[3]
	serverURL := os.Args[4]

	for {
		time.Sleep(15 * time.Second)
		exists, err := process.PidExists(int32(pid))
		if err != nil || !exists {
			log.Printf("watch: parent PID %d dead, shutting down machine %s", pid, machineCode)
			httpPost(serverURL+"/api/machines/"+machineCode+"/remote/shutdown", nil)
			return
		}
	}
}
