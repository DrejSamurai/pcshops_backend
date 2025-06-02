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
	GetFilteredProducts(category, manufacturer, store, minPrice, maxPrice, title, pageStr, pageSizeStr string) ([]*Product, int, error)
	GetUniqueManufacturers() ([]string, error)
	GetManufacturersByCategory(category string) ([]string, error)
	GetUniqueStores() ([]string, error)
	GetProductByID(id int) (*Product, error)
	CreateUser(*User) error
	GetUserByEmail(email string) (*User, error)
	CreateConfiguration(userID int, name string) (int, error)
	AddProductToConfiguration(configID, productID int) error
	RemoveProductFromConfiguration(configID, productID int) error
	GetProductsByConfigurationID(configID int) ([]*Product, error)
	GetConfigurationsByUserID(userID int) ([]*ComputerConfiguration, error)
	GetRandomProducts(limit int) ([]*Product, error)
}

type PostgressStore struct {
	db *sql.DB
}

func (s *PostgressStore) CreateUser(user *User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (email, password) VALUES ($1, $2)
	`, user.Email, user.Password)
	return err
}

func (s *PostgressStore) GetUserByEmail(email string) (*User, error) {
	row := s.db.QueryRow("SELECT id, email, password FROM users WHERE email = $1", email)
	user := new(User)
	err := row.Scan(&user.ID, &user.Email, &user.Password)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func NewPostgressStore() (*PostgressStore, error) {
	connStr := "host=db user=postgres dbname=postgres password=pcshops sslmode=disable"
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
	if err := s.createProductsTable(); err != nil {
		return err
	}
	if err := s.createUserTable(); err != nil {
		return err
	}
	return nil
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

func (s *PostgressStore) createUserTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL
		)
	`)
	return err
}

func (s *PostgressStore) CreateComputerConfigurationsTable() error {
	query := `
    CREATE TABLE IF NOT EXISTS computer_configurations (
        id SERIAL PRIMARY KEY,
        user_id INTEGER NOT NULL REFERENCES users(id),
        name TEXT NOT NULL,
        total_price BIGINT NOT NULL DEFAULT 0,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
    `
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgressStore) CreateConfigurationItemsTable() error {
	query := `
    CREATE TABLE IF NOT EXISTS configuration_items (
        id SERIAL PRIMARY KEY,
        configuration_id INTEGER NOT NULL REFERENCES computer_configurations(id) ON DELETE CASCADE,
        product_id INTEGER NOT NULL REFERENCES products(id),
        UNIQUE(configuration_id, product_id)
    );
    `
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

func (s *PostgressStore) CreateConfiguration(userID int, name string) (int, error) {
	var configID int
	err := s.db.QueryRow(`
        INSERT INTO computer_configurations (user_id, name)
        VALUES ($1, $2)
        RETURNING id
    `, userID, name).Scan(&configID)
	if err != nil {
		return 0, err
	}
	return configID, nil
}

func (s *PostgressStore) AddProductToConfiguration(configID, productID int) error {
	_, err := s.db.Exec(`
        INSERT INTO configuration_items (configuration_id, product_id)
        VALUES ($1, $2)
        ON CONFLICT DO NOTHING
    `, configID, productID)
	return err
}

func (s *PostgressStore) RemoveProductFromConfiguration(configID, productID int) error {
	_, err := s.db.Exec(`
        DELETE FROM configuration_items
        WHERE configuration_id = $1 AND product_id = $2
    `, configID, productID)
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

func (s *PostgressStore) GetFilteredProducts(category, manufacturer, store, minPrice, maxPrice, title, pageStr, pageSizeStr string) ([]*Product, int, error) {
	baseQuery := " FROM products WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	filterQuery := ""

	if category != "" {
		filterQuery += fmt.Sprintf(" AND category = $%d", argIndex)
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
		filterQuery += fmt.Sprintf(" AND manufacturer IN (%s)", strings.Join(placeholders, ","))
	}
	if store != "" {
		filterQuery += fmt.Sprintf(" AND store = $%d", argIndex)
		args = append(args, store)
		argIndex++
	}
	if minPrice != "" {
		filterQuery += fmt.Sprintf(" AND price >= $%d", argIndex)
		args = append(args, minPrice)
		argIndex++
	}
	if maxPrice != "" {
		filterQuery += fmt.Sprintf(" AND price <= $%d", argIndex)
		args = append(args, maxPrice)
		argIndex++
	}
	if title != "" {
		filterQuery += fmt.Sprintf(" AND title ILIKE $%d", argIndex)
		args = append(args, "%"+title+"%")
		argIndex++
	}

	countQuery := "SELECT COUNT(*)" + baseQuery + filterQuery
	var totalCount int
	err := s.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
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

	filteredArgs := make([]interface{}, len(args))
	copy(filteredArgs, args)
	filteredArgs = append(filteredArgs, pageSize, offset)

	dataQuery := "SELECT *" + baseQuery + filterQuery + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)

	rows, err := s.db.Query(dataQuery, filteredArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	products := []*Product{}
	for rows.Next() {
		product, err := scanIntoProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, product)
	}

	return products, totalCount, nil
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

func (s *PostgressStore) GetProductByID(id int) (*Product, error) {
	row := s.db.QueryRow("SELECT * FROM products WHERE id = $1", id)
	product := new(Product)
	err := row.Scan(
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
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return product, nil
}

func (s *PostgressStore) GetProductsByConfigurationID(configID int) ([]*Product, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.title, p.description, p.price,
		       p.category, p.code, p.image, p.link, p.manufacturer, p.store, p.warranty
		FROM products p
		JOIN configuration_items ci ON ci.product_id = p.id
		WHERE ci.configuration_id = $1
	`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Description, &p.Price,
			&p.Category, &p.Code, &p.Image, &p.Link, &p.Manufacturer, &p.Store, &p.Warranty,
		); err != nil {
			return nil, err
		}
		products = append(products, &p)
	}
	return products, nil
}

func (s *PostgressStore) GetConfigurationsByUserID(userID int) ([]*ComputerConfiguration, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name
		FROM computer_configurations
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*ComputerConfiguration
	for rows.Next() {
		var c ComputerConfiguration
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name); err != nil {
			return nil, err
		}

		products, err := s.GetProductsByConfigurationID(c.ID)
		if err != nil {
			return nil, err
		}
		c.Products = products

		configs = append(configs, &c)
	}
	return configs, nil
}

func (s *PostgressStore) GetRandomProducts(limit int) ([]*Product, error) {
	query := `
        SELECT * FROM products
        ORDER BY RANDOM()
        LIMIT $1
    `
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product, err := scanIntoProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, nil
}
