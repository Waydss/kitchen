package catalog

import "time"

type Restaurant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	Images      []RestaurantImage `json:"images"`
}

type RestaurantSummary struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	CoverImageURL *string   `json:"cover_image_url"`
	CreatedAt     time.Time `json:"created_at"`
}

type RestaurantImage struct {
	URL      string `json:"url"`
	Position int    `json:"position"`
}

type Menu struct {
	RestaurantID string         `json:"restaurant_id"`
	Categories   []MenuCategory `json:"categories"`
}

type MenuCategory struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Position int        `json:"position"`
	Items    []MenuItem `json:"items"`
}

type MenuItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       int64   `json:"price"`
	Available   bool    `json:"available"`
	ImageURL    *string `json:"image_url"`
}
