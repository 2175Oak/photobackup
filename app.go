package main

import (
	"context"
	"photo-backup/service"
)

// App struct
type App struct {
	photoService *service.PhotoService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup passes the Wails context to the service's runtime calls.
func (a *App) startup(ctx context.Context) {
	a.photoService.SetContext(ctx)
}
