package mongo

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SubscriptionRepository struct {
	collection *mongo.Collection
}

func NewSubscriptionRepository(db *mongo.Database) *SubscriptionRepository {
	return &SubscriptionRepository{
		collection: db.Collection("subscriptions"),
	}
}

func (r *SubscriptionRepository) Save(ctx context.Context, sub *domain.Subscription) error {
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, bson.M{"id": sub.ID}, bson.M{"$set": sub}, opts)
	return err
}

func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&sub)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.collection.FindOne(ctx, bson.M{"stripeSubscriptionId": stripeSubID}).Decode(&sub)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) GetByStripeCustomerID(ctx context.Context, stripeCustomerID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.collection.FindOne(ctx, bson.M{"stripeCustomerId": stripeCustomerID}).Decode(&sub)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) GetActiveSubscriptions(ctx context.Context) ([]*domain.Subscription, error) {
	var subs []*domain.Subscription
	cursor, err := r.collection.Find(ctx, bson.M{
		"status": bson.M{"$in": []domain.SubscriptionStatus{domain.StatusActive, domain.StatusTrialing}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}
