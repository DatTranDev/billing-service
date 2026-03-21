package mongo

import (
	"context"
	"time"
	"github.com/teachingassistant/billing-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UsageRepository struct {
	collection *mongo.Collection
}

func NewUsageRepository(db *mongo.Database) *UsageRepository {
	return &UsageRepository{
		collection: db.Collection("usage_tracking"),
	}
}

func (r *UsageRepository) Log(ctx context.Context, event domain.UsageEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	_, err := r.collection.InsertOne(ctx, event)
	return err
}

func (r *UsageRepository) GetTotalUsage(ctx context.Context, userID string, usageType domain.UsageType, since time.Time) (int, error) {
	filter := bson.M{
		"userId":    userID,
		"type":      usageType,
		"timestamp": bson.M{"$gte": since},
	}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	total := 0
	for cursor.Next(ctx) {
		var event domain.UsageEvent
		if err := cursor.Decode(&event); err != nil {
			return 0, err
		}
		total += event.Amount
	}
	return total, nil
}

func (r *UsageRepository) ListUsage(ctx context.Context, userID string, usageType domain.UsageType, limit int) ([]domain.UsageEvent, error) {
	filter := bson.M{"userId": userID}
	if usageType != "" {
		filter["type"] = usageType
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []domain.UsageEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}
