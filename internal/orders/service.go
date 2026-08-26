package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repository       Repository
	restaurantClient RestaurantClient
}

func NewService(repository Repository, restaurantClient RestaurantClient) *Service {
	return &Service{repository: repository, restaurantClient: restaurantClient}
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (CreateResult, error) {
	if err := validateCreateCommand(command); err != nil {
		return CreateResult{}, err
	}

	existing, err := s.repository.FindByIdempotencyKey(ctx, command.CustomerID, command.IdempotencyKey)
	switch {
	case err == nil:
		if !sameRequest(existing, command) {
			return CreateResult{}, ErrIdempotencyConflict
		}
		return s.submitPending(ctx, existing, false)
	case !errors.Is(err, ErrNotFound):
		return CreateResult{}, fmt.Errorf("find idempotent order: %w", err)
	}

	order, created, err := s.repository.CreatePending(ctx, command)
	if err != nil {
		return CreateResult{}, fmt.Errorf("create pending order: %w", err)
	}
	if !created && !sameRequest(order, command) {
		return CreateResult{}, ErrIdempotencyConflict
	}
	return s.submitPending(ctx, order, created)
}

func (s *Service) Get(ctx context.Context, id string) (Order, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Order{}, ErrInvalidRequest
	}
	order, err := s.repository.Get(ctx, id)
	if err != nil {
		return Order{}, fmt.Errorf("get order: %w", err)
	}
	return order, nil
}

func (s *Service) submitPending(ctx context.Context, order Order, created bool) (CreateResult, error) {
	if order.Status != StatusPending {
		return CreateResult{Order: order, Created: created}, nil
	}

	decision, err := s.restaurantClient.SubmitOrder(ctx, order)
	if err != nil {
		return CreateResult{Order: order, Created: created, Pending: true}, ErrPartnerUnavailable
	}
	order, err = s.repository.ApplyRestaurantDecision(ctx, order.ID, decision)
	if err != nil {
		return CreateResult{}, fmt.Errorf("apply restaurant decision: %w", err)
	}
	return CreateResult{Order: order, Created: created}, nil
}

func validateCreateCommand(command CreateCommand) error {
	if _, err := uuid.Parse(command.CustomerID); err != nil {
		return ErrInvalidRequest
	}
	if _, err := uuid.Parse(command.RestaurantID); err != nil {
		return ErrInvalidRequest
	}
	if len(command.IdempotencyKey) < 8 || len(command.IdempotencyKey) > 128 {
		return ErrInvalidRequest
	}
	if len(command.Items) == 0 || len(command.Items) > 50 {
		return ErrInvalidRequest
	}
	seen := make(map[string]struct{}, len(command.Items))
	for _, item := range command.Items {
		if _, err := uuid.Parse(item.MenuItemID); err != nil || item.Quantity < 1 || item.Quantity > 20 {
			return ErrInvalidRequest
		}
		if _, exists := seen[item.MenuItemID]; exists {
			return ErrInvalidRequest
		}
		seen[item.MenuItemID] = struct{}{}
	}
	return nil
}

func sameRequest(order Order, command CreateCommand) bool {
	if order.CustomerID != command.CustomerID || order.RestaurantID != command.RestaurantID || len(order.Items) != len(command.Items) {
		return false
	}
	quantities := make(map[string]int, len(order.Items))
	for _, item := range order.Items {
		quantities[item.MenuItemID] = item.Quantity
	}
	for _, item := range command.Items {
		if quantities[item.MenuItemID] != item.Quantity {
			return false
		}
	}
	return true
}
