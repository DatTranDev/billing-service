package domain

import (
	"context"
)

type User struct {
	ID    string `bson:"_id"`
	Name  string `bson:"name"`
	Email string `bson:"email"`
}

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*User, error)
}
