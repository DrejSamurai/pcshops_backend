package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
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
	router.HandleFunc("/register", makeHTTPHandleFunc(s.handleRegister)).Methods("POST")
	router.HandleFunc("/login", makeHTTPHandleFunc(s.handleLogin)).Methods("POST")
	router.HandleFunc("/api/youtube", handleYouTubeSearch)
	router.HandleFunc("/configurations", makeHTTPHandleFunc(s.handleCreateConfiguration)).Methods("POST")
	router.HandleFunc("/configurations/{id}/products", makeHTTPHandleFunc(s.handleAddProductToConfiguration)).Methods("POST")
	router.HandleFunc("/configurations/{id}/products/{productID}", makeHTTPHandleFunc(s.handleRemoveProductFromConfiguration)).Methods("DELETE")
	router.HandleFunc("/users/{userID}/configurations", makeHTTPHandleFunc(s.handleGetConfigurationsByUser)).Methods("GET")
	router.HandleFunc("/products/random", makeHTTPHandleFunc(s.handleGetRandomProducts)).Methods("GET")

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

func (s *APIServer) handleRegister(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	user := &User{
		Email:    req.Email,
		Password: string(hashedPassword),
	}
	if err := s.store.CreateUser(user); err != nil {
		return fmt.Errorf("error creating user: %w", err)
	}
	return WriteJSON(w, http.StatusCreated, map[string]string{"message": "user created"})
}

func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	user, err := s.store.GetUserByEmail(req.Email)
	if err != nil {
		return fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return fmt.Errorf("invalid credentials")
	}

	token, err := GenerateJWT(user.ID)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	return WriteJSON(w, http.StatusOK, map[string]string{"token": token})
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

func handleYouTubeSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		log.Println("YOUTUBE_API_KEY is not set")
		http.Error(w, "API key not configured", http.StatusInternalServerError)
		return
	}

	youtubeURL := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/search?part=snippet&type=video&maxResults=5&q=%s&key=%s",
		url.QueryEscape(query), apiKey,
	)

	resp, err := http.Get(youtubeURL)
	if err != nil {
		log.Println("Error fetching from YouTube API:", err)
		http.Error(w, "Failed to fetch from YouTube API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading YouTube response body:", err)
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != 200 {
		log.Printf("YouTube API returned status %d: %s\n", resp.StatusCode, string(body))
		http.Error(w, "YouTube API error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func (s *APIServer) handleCreateConfiguration(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		UserID int    `json:"userID"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	configID, err := s.store.CreateConfiguration(req.UserID, req.Name)
	if err != nil {
		return fmt.Errorf("failed to create configuration: %w", err)
	}

	return WriteJSON(w, http.StatusCreated, map[string]int{"configID": configID})
}

func (s *APIServer) handleAddProductToConfiguration(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	configIDStr := vars["id"]
	var configID int
	_, err := fmt.Sscanf(configIDStr, "%d", &configID)
	if err != nil {
		return fmt.Errorf("invalid configuration ID")
	}

	var req struct {
		ProductID int `json:"productID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	if err := s.store.AddProductToConfiguration(configID, req.ProductID); err != nil {
		return fmt.Errorf("failed to add product to configuration: %w", err)
	}

	return WriteJSON(w, http.StatusOK, map[string]string{"message": "product added"})
}

func (s *APIServer) handleRemoveProductFromConfiguration(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	configIDStr := vars["id"]
	productIDStr := vars["productID"]
	var configID, productID int
	_, err := fmt.Sscanf(configIDStr, "%d", &configID)
	if err != nil {
		return fmt.Errorf("invalid configuration ID")
	}
	_, err = fmt.Sscanf(productIDStr, "%d", &productID)
	if err != nil {
		return fmt.Errorf("invalid product ID")
	}

	if err := s.store.RemoveProductFromConfiguration(configID, productID); err != nil {
		return fmt.Errorf("failed to remove product from configuration: %w", err)
	}

	return WriteJSON(w, http.StatusOK, map[string]string{"message": "product removed"})
}

func (s *APIServer) handleGetConfigurationsByUser(w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	var userID int
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		return fmt.Errorf("invalid user ID")
	}

	configs, err := s.store.GetConfigurationsByUserID(userID)
	if err != nil {
		return fmt.Errorf("could not get configurations: %w", err)
	}

	return WriteJSON(w, http.StatusOK, configs)
}

func (s *APIServer) handleGetRandomProducts(w http.ResponseWriter, r *http.Request) error {
	products, err := s.store.GetRandomProducts(12)
	if err != nil {
		return fmt.Errorf("failed to fetch random products: %w", err)
	}
	return WriteJSON(w, http.StatusOK, products)
}
