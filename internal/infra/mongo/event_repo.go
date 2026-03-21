package mongo

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type StripeEventRepository struct {
	collection *mongo.Collection
}

func NewStripeEventRepository(db *mongo.Database) *StripeEventRepository {
	return &StripeEventRepository{
		collection: db.Collection("stripe_events"),
	}
}

func (r *StripeEventRepository) Exists(ctx context.Context, id string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"id": id})
	return count > 0, err
}

func (r *StripeEventRepository) Save(ctx context.Context, id string) error {
	_, err := r.collection.InsertOne(ctx, bson.M{"id": id})
	return err
}
