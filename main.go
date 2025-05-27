package main

import (
	"log"

	"github.com/joho/godotenv"
)

func main() {
	store, err := NewPostgressStore()
	if err != nil {
		log.Fatal(err)
	}

	err = godotenv.Load("private.env")
	if err != nil {
		log.Fatalf("Error loading private.env file: %v", err)
	}

	if err := store.createProductsTable(); err != nil {
		log.Fatal("Could not create products table:", err)
	}

	if err := store.createUserTable(); err != nil {
		log.Fatal("Could not create users table:", err)
	}

	if err := store.CreateComputerConfigurationsTable(); err != nil {
		log.Fatal("Could not create computer configuration table:", err)
	}
	if err := store.CreateConfigurationItemsTable(); err != nil {
		log.Fatal("Could not create computer item configuration table:", err)
	}

	err = ImportProductsFromCSV(store, "products.csv")
	if err != nil {
		log.Fatalf("CSV import failed: %v", err)
	}

	server := NewAPIServer(":3000", store)
	server.Run()
}
