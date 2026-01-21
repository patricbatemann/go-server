package database

import (
	"log"
	"os"

	"github.com/supabase-community/supabase-go"
)

var Client *supabase.Client

func InitDB() {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_KEY must be set in environment variables")
	}

	var err error
	Client, err = supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Supabase client: %v", err)
	}

	log.Println("Supabase client initialized successfully.")
}

func GetClient() *supabase.Client {
	return Client
}
