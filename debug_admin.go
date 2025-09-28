package main

import (
	"fmt"
	"log"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("AdminUsername: '%s'\n", cfg.AdminUsername)
	fmt.Printf("AdminPassword: '%s'\n", cfg.AdminPassword)
	fmt.Printf("AdminSessionSecret: '%s'\n", cfg.AdminSessionSecret)
	fmt.Printf("AdminEnabled(): %v\n", cfg.AdminEnabled())
}
