package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListRestaurants(ctx context.Context) ([]RestaurantSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.name, r.description, r.status, cover.url, r.created_at
		FROM restaurants r
		LEFT JOIN restaurant_images cover
		  ON cover.restaurant_id = r.id AND cover.is_cover
		WHERE r.status <> 'disabled'
		ORDER BY r.name, r.id`)
	if err != nil {
		return nil, fmt.Errorf("query restaurants: %w", err)
	}
	defer rows.Close()

	restaurants := make([]RestaurantSummary, 0)
	for rows.Next() {
		var restaurant RestaurantSummary
		var coverImageURL pgtype.Text
		if err := rows.Scan(
			&restaurant.ID,
			&restaurant.Name,
			&restaurant.Description,
			&restaurant.Status,
			&coverImageURL,
			&restaurant.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan restaurant: %w", err)
		}
		if coverImageURL.Valid {
			value := coverImageURL.String
			restaurant.CoverImageURL = &value
		}
		restaurants = append(restaurants, restaurant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate restaurants: %w", err)
	}
	return restaurants, nil
}

func (r *PostgresRepository) GetRestaurant(ctx context.Context, id string) (Restaurant, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Restaurant{}, fmt.Errorf("begin restaurant read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var restaurant Restaurant
	err = tx.QueryRow(ctx, `
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
	restaurant.Images = make([]RestaurantImage, 0)
	rows, err := tx.Query(ctx, `
		SELECT url, position
		FROM restaurant_images
		WHERE restaurant_id = $1
		ORDER BY position, id`, id)
	if err != nil {
		return Restaurant{}, fmt.Errorf("query restaurant images: %w", err)
	}
	for rows.Next() {
		var image RestaurantImage
		if err = rows.Scan(&image.URL, &image.Position); err != nil {
			rows.Close()
			return Restaurant{}, fmt.Errorf("scan restaurant image: %w", err)
		}
		restaurant.Images = append(restaurant.Images, image)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Restaurant{}, fmt.Errorf("iterate restaurant images: %w", err)
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return Restaurant{}, fmt.Errorf("commit restaurant read transaction: %w", err)
	}
	return restaurant, nil
}

func (r *PostgresRepository) GetMenu(ctx context.Context, restaurantID string) ([]MenuCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, c.position,
		       i.id, i.name, i.description, i.price, i.is_available, i.image_url
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
		var itemImageURL pgtype.Text
		if err := rows.Scan(
			&category.ID, &category.Name, &category.Position,
			&itemID, &itemName, &itemDescription, &itemPrice, &itemAvailable, &itemImageURL,
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
			item := MenuItem{
				ID:          *itemID,
				Name:        *itemName,
				Description: *itemDescription,
				Price:       *itemPrice,
				Available:   *itemAvailable,
			}
			if itemImageURL.Valid {
				value := itemImageURL.String
				item.ImageURL = &value
			}
			categories[index].Items = append(categories[index].Items, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menu: %w", err)
	}
	return categories, nil
}
