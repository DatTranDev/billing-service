package mongo

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LedgerRepository struct {
	collection *mongo.Collection
}

func NewLedgerRepository(db *mongo.Database) *LedgerRepository {
	return &LedgerRepository{
		collection: db.Collection("ledger"),
	}
}

func (r *LedgerRepository) Save(ctx context.Context, entry *domain.LedgerEntry) error {
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"stripeId": entry.StripeID, "type": entry.Type},
		bson.M{"$set": entry},
		opts,
	)
	return err
}

func (r *LedgerRepository) GetByUserID(ctx context.Context, userID string) ([]*domain.LedgerEntry, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entries []*domain.LedgerEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
