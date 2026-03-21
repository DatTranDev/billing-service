package mongo

import (
	"context"
	"github.com/teachingassistant/billing-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WalletRepository struct {
	collection *mongo.Collection
}

func NewWalletRepository(db *mongo.Database) *WalletRepository {
	return &WalletRepository{
		collection: db.Collection("credit_wallet"),
	}
}

func (r *WalletRepository) GetByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	var wallet domain.Wallet
	err := r.collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet)
	if err == mongo.ErrNoDocuments {
		return &domain.Wallet{UserID: userID}, nil
	}
	return &wallet, err
}

func (r *WalletRepository) Update(ctx context.Context, wallet *domain.Wallet) error {
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"userId": wallet.UserID},
		bson.M{"$set": wallet},
		opts,
	)
	return err
}

func (r *WalletRepository) IncrementStorage(ctx context.Context, userID string, deltaBytes int) error {
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"userId": userID},
		bson.M{"$inc": bson.M{"storageUsedBytes": deltaBytes}},
		opts,
	)
	return err
}
