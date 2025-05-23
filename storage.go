package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type Storage interface {
	CreateProduct(*Product) error
	GetProducts() ([]*Product, error)
	GetFilteredProducts(category, manufacturer, store, minPrice, maxPrice, title, pageStr, pageSizeStr string) ([]*Product, error)
	GetUniqueManufacturers() ([]string, error)
	GetManufacturersByCategory(category string) ([]string, error)
	GetUniqueStores() ([]string, error)
}

type PostgressStore struct {
	db *sql.DB
}

func NewPostgressStore() (*PostgressStore, error) {
	connStr := "user=postgres dbname=postgres password=pcshops sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgressStore{
		db: db,
	}, nil
}

func (s *PostgressStore) Init() error {
	return s.createProductsTable()
}

func (s *PostgressStore) createProductsTable() error {
	query := `CREATE TABLE IF NOT EXISTS products (
		id SERIAL PRIMARY KEY,
		title TEXT,
		manufacturer TEXT,
		price BIGINT,
		code TEXT,
		warranty BIGINT,
		link TEXT,
		category TEXT,
		description TEXT,
		image TEXT,
		store TEXT
	)`

	_, err := s.db.Exec(query)
	return err
}

func (s *PostgressStore) CreateProduct(p *Product) error {
	_, err := s.db.Exec(`
		INSERT INTO products (title, manufacturer, price, code, warranty, link, category, description, image, store)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, p.Title, p.Manufacturer, p.Price, p.Code, p.Warranty, p.Link, p.Category, p.Description, p.Image, p.Store)

	return err
}

func (s *PostgressStore) GetProducts() ([]*Product, error) {
	rows, err := s.db.Query("SELECT * FROM products")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []*Product{}
	for rows.Next() {
		product, err := scanIntoProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	return products, nil
}

func scanIntoProduct(rows *sql.Rows) (*Product, error) {
	product := new(Product)
	err := rows.Scan(
		&product.ID,
		&product.Title,
		&product.Manufacturer,
		&product.Price,
		&product.Code,
		&product.Warranty,
		&product.Link,
		&product.Category,
		&product.Description,
		&product.Image,
		&product.Store,
	)

	return product, err
}

func (s *PostgressStore) GetFilteredProducts(category, manufacturer, store, minPrice, maxPrice, title, pageStr, pageSizeStr string) ([]*Product, error) {
	query := "SELECT * FROM products WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, category)
		argIndex++
	}
	if manufacturer != "" {
		manufacturers := strings.Split(manufacturer, ",")
		placeholders := []string{}
		for _, m := range manufacturers {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			args = append(args, strings.TrimSpace(m))
			argIndex++
		}
		query += fmt.Sprintf(" AND manufacturer IN (%s)", strings.Join(placeholders, ","))
	}
	if store != "" {
		query += fmt.Sprintf(" AND store = $%d", argIndex)
		args = append(args, store)
		argIndex++
	}
	if minPrice != "" {
		query += fmt.Sprintf(" AND price >= $%d", argIndex)
		args = append(args, minPrice)
		argIndex++
	}
	if maxPrice != "" {
		query += fmt.Sprintf(" AND price <= $%d", argIndex)
		args = append(args, maxPrice)
		argIndex++
	}
	if title != "" {
		query += fmt.Sprintf(" AND title ILIKE $%d", argIndex)
		args = append(args, "%"+title+"%")
		argIndex++
	}

	page := 1
	pageSize := 20
	if pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
	}
	if pageSizeStr != "" {
		fmt.Sscanf(pageSizeStr, "%d", &pageSize)
	}
	offset := (page - 1) * pageSize

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []*Product{}
	for rows.Next() {
		product, err := scanIntoProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	return products, nil
}

func (s *PostgressStore) GetUniqueManufacturers() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT manufacturer FROM products WHERE manufacturer IS NOT NULL AND manufacturer != ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var manufacturers []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		manufacturers = append(manufacturers, m)
	}
	return manufacturers, nil
}

func (s *PostgressStore) GetManufacturersByCategory(category string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT manufacturer
		FROM products
		WHERE category = $1 AND manufacturer IS NOT NULL AND manufacturer <> ''
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var manufacturers []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		manufacturers = append(manufacturers, m)
	}

	return manufacturers, nil
}

func (s *PostgressStore) GetUniqueStores() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT store FROM products WHERE store IS NOT NULL AND store != ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stores []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		stores = append(stores, m)
	}
	return stores, nil
}
