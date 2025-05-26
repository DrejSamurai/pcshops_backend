package main

import (
	"log"
)

func main() {
	store, err := NewPostgressStore()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.createProductsTable(); err != nil {
		log.Fatal("Could not create products table:", err)
	}

	if err := store.createUserTable(); err != nil {
		log.Fatal("Could not create users table:", err)
	}

	err = ImportProductsFromCSV(store, "products.csv")
	if err != nil {
		log.Fatalf("CSV import failed: %v", err)
	}

	server := NewAPIServer(":3000", store)
	server.Run()
}
