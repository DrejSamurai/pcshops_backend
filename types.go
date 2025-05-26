package main

type Product struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Manufacturer string `json:"manufacturer"`
	Price        int64  `json:"price"`
	Code         string `json:"code"`
	Warranty     int64  `json:"warranty"`
	Link         string `json:"link"`
	Category     string `json:"category"`
	Description  string `json:"description"`
	Image        string `json:"image"`
	Store        string `json:"store"`
}

type User struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Password string `json:"-"`
}
