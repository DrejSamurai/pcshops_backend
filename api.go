package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

type APIServer struct {
	listenAddr string
	store      Storage
}

func (s *APIServer) Run() {
	router := mux.NewRouter()

	router.HandleFunc("/product", makeHTTPHandleFunc(s.handleProduct))
	router.HandleFunc("/products", makeHTTPHandleFunc(s.handleFilteredProducts))
	router.HandleFunc("/image-proxy", makeHTTPHandleFunc(s.handleImageProxy))
	router.HandleFunc("/manufacturers", makeHTTPHandleFunc(s.handleGetManufacturers))
	router.HandleFunc("/stores", makeHTTPHandleFunc(s.handleGetStores))
	router.HandleFunc("/product/{id}", makeHTTPHandleFunc(s.handleGetProductById))

	corsRouter := corsMiddleware(router)

	log.Println("JSON API server running on port: ", s.listenAddr)
	http.ListenAndServe(s.listenAddr, corsRouter)
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) handleProduct(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetProduct(w, r)
	}
	return fmt.Errorf("method not allowed %s", r.Method)
}

func (s *APIServer) handleGetProductById(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		return fmt.Errorf("method not allowed %s", r.Method)
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	var id int
	_, err := fmt.Sscanf(idStr, "%d", &id)
	if err != nil {
		return fmt.Errorf("invalid product ID")
	}

	product, err := s.store.GetProductByID(id)
	if err != nil {
		return fmt.Errorf("could not fetch product: %w", err)
	}

	return WriteJSON(w, http.StatusOK, product)
}

func (s *APIServer) handleGetProduct(w http.ResponseWriter, r *http.Request) error {
	products, err := s.store.GetProducts()
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, products)
}

func (s *APIServer) handleFilteredProducts(w http.ResponseWriter, r *http.Request) error {
	category := r.URL.Query().Get("category")
	manufacturer := r.URL.Query().Get("manufacturer")
	store := r.URL.Query().Get("store")
	minPrice := r.URL.Query().Get("minPrice")
	maxPrice := r.URL.Query().Get("maxPrice")
	title := r.URL.Query().Get("title")
	page := r.URL.Query().Get("page")
	pageSize := r.URL.Query().Get("pageSize")

	products, totalCount, err := s.store.GetFilteredProducts(category, manufacturer, store, minPrice, maxPrice, title, page, pageSize)
	if err != nil {
		return err
	}

	response := struct {
		Data       []*Product `json:"data"`
		TotalCount int        `json:"totalCount"`
	}{
		Data:       products,
		TotalCount: totalCount,
	}

	return WriteJSON(w, http.StatusOK, response)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type APIError struct {
	Error string
}

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) handleImageProxy(w http.ResponseWriter, r *http.Request) error {
	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		http.Error(w, "Missing 'url' parameter", http.StatusBadRequest)
		return nil
	}

	parsedURL, err := url.Parse(imageURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		http.Error(w, "Invalid image URL", http.StatusBadRequest)
		return nil
	}

	resp, err := http.Get(imageURL)
	if err != nil {
		http.Error(w, "Failed to fetch image", http.StatusInternalServerError)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Image not accessible", http.StatusBadGateway)
		return nil
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, resp.Body)
	return copyErr
}

func (s *APIServer) handleGetManufacturers(w http.ResponseWriter, r *http.Request) error {
	category := r.URL.Query().Get("category")

	var manufacturers []string
	var err error

	if category != "" {
		manufacturers, err = s.store.GetManufacturersByCategory(category)
	} else {
		manufacturers, err = s.store.GetUniqueManufacturers()
	}

	if err != nil {
		return fmt.Errorf("failed to fetch manufacturers: %w", err)
	}

	return WriteJSON(w, http.StatusOK, manufacturers)
}

func (s *APIServer) handleGetStores(w http.ResponseWriter, r *http.Request) error {
	stores, err := s.store.GetUniqueStores()
	if err != nil {
		return fmt.Errorf("failed to fetch stores: %w", err)
	}
	return WriteJSON(w, http.StatusOK, stores)
}
