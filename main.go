package main

import (
	"embed"

	"photo-backup/repository"
	"photo-backup/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// เชื่อมต่อฐานข้อมูล
	db, err := repository.NewDbConnection()
	if err != nil {
		println("Error:", err.Error())
		return
	}
	println("Database connected successfully")

	photoService := service.NewPhotoService(db)
	app.photoService = photoService

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "photo-backup",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		MinWidth:         860,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 243, G: 245, B: 241, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			photoService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
