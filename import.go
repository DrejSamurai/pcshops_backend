package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

func ImportProductsFromCSV(store Storage, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("could not read CSV: %w", err)
	}

	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) != 10 {
			return fmt.Errorf("row %d has wrong number of columns", i+1)
		}

		price, _ := strconv.ParseInt(row[2], 10, 64)
		warranty, _ := strconv.ParseInt(row[4], 10, 64)

		product := &Product{
			Title:        row[0],
			Manufacturer: row[1],
			Price:        price,
			Code:         row[3],
			Warranty:     warranty,
			Link:         row[5],
			Category:     row[6],
			Description:  row[7],
			Image:        row[8],
			Store:        row[9],
		}

		if err := store.CreateProduct(product); err != nil {
			fmt.Printf("failed to insert product at row %d: %v\n", i+1, err)
		}
	}

	return nil
}
