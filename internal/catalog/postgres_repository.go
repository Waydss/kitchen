package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListRestaurants(ctx context.Context) ([]Restaurant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, status, created_at
		FROM restaurants
		WHERE status <> 'disabled'
		ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("query restaurants: %w", err)
	}
	defer rows.Close()

	restaurants := make([]Restaurant, 0)
	for rows.Next() {
		var restaurant Restaurant
		if err := rows.Scan(&restaurant.ID, &restaurant.Name, &restaurant.Description, &restaurant.Status, &restaurant.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan restaurant: %w", err)
		}
		restaurants = append(restaurants, restaurant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate restaurants: %w", err)
	}
	return restaurants, nil
}

func (r *PostgresRepository) GetRestaurant(ctx context.Context, id string) (Restaurant, error) {
	var restaurant Restaurant
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, status, created_at
		FROM restaurants
		WHERE id = $1 AND status <> 'disabled'`, id).
		Scan(&restaurant.ID, &restaurant.Name, &restaurant.Description, &restaurant.Status, &restaurant.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Restaurant{}, ErrNotFound
	}
	if err != nil {
		return Restaurant{}, fmt.Errorf("query restaurant: %w", err)
	}
	return restaurant, nil
}

func (r *PostgresRepository) GetMenu(ctx context.Context, restaurantID string) ([]MenuCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, c.position,
		       i.id, i.name, i.description, i.price, i.is_available
		FROM menu_categories c
		LEFT JOIN menu_items i
		  ON i.restaurant_id = c.restaurant_id AND i.category_id = c.id
		WHERE c.restaurant_id = $1
		ORDER BY c.position, c.id, i.name, i.id`, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("query menu: %w", err)
	}
	defer rows.Close()

	categories := make([]MenuCategory, 0)
	categoryIndexes := make(map[string]int)
	for rows.Next() {
		var category MenuCategory
		var itemID, itemName, itemDescription *string
		var itemPrice *int64
		var itemAvailable *bool
		if err := rows.Scan(
			&category.ID, &category.Name, &category.Position,
			&itemID, &itemName, &itemDescription, &itemPrice, &itemAvailable,
		); err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}

		index, exists := categoryIndexes[category.ID]
		if !exists {
			category.Items = make([]MenuItem, 0)
			categories = append(categories, category)
			index = len(categories) - 1
			categoryIndexes[category.ID] = index
		}
		if itemID != nil {
			categories[index].Items = append(categories[index].Items, MenuItem{
				ID:          *itemID,
				Name:        *itemName,
				Description: *itemDescription,
				Price:       *itemPrice,
				Available:   *itemAvailable,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menu: %w", err)
	}
	return categories, nil
}
