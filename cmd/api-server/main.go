package main

import (
	"context"
	"log"
	"os"

	"github.com/sentinelmesh/sentinelmesh/internal/bootstrap"
)

func main() {
	log.Println("Starting SentinelMesh API Server (Stage 5)")
	
	if err := bootstrap.Run(context.Background()); err != nil {
		log.Printf("Fatal server error: %v", err)
		os.Exit(1)
	}
}
