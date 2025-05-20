package main

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type Storage interface {
	CreateProduct(*Product) error
	GetProducts() ([]*Product, error)
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
