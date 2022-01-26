package main

type Shop struct {
	Id           int    `json:"id"`
	Image        string `json:"image"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	WorkingHours struct {
		Closing string `json:"closing"`
		Opening string `json:"opening"`
	} `json:"workingHours"`
	Menu []Product
}

type Product struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Price       float64  `json:"price"`
	Image       string   `json:"image"`
	Type        string   `json:"type"`
	Ingredients []string `json:"ingredients"`
}
