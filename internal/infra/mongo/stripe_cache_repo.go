package mongo

import (
	"context"
	"time"

	"github.com/teachingassistant/billing-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StripeCacheRepository struct {
	collection *mongo.Collection
}

func NewStripeCacheRepository(db *mongo.Database) *StripeCacheRepository {
	return &StripeCacheRepository{
		collection: db.Collection("stripecache"),
	}
}

func (r *StripeCacheRepository) Save(ctx context.Context, entry *domain.StripeCacheEntry) error {
	entry.LastUpdate = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"id": entry.ID},
		bson.M{"$set": entry},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *StripeCacheRepository) GetByID(ctx context.Context, id string) (*domain.StripeCacheEntry, error) {
	var entry domain.StripeCacheEntry
	err := r.collection.FindOne(ctx, bson.M{"id": id}).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &entry, err
}

func (r *StripeCacheRepository) Delete(ctx context.Context, id string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"id": id})
	return err
}
