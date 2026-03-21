package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
)

type CreditPack struct {
	ID              string    `bson:"id"`
	TotalAmount     int       `bson:"totalAmount"`
	RemainingAmount int       `bson:"remainingAmount"`
	Type            string    `bson:"type"` // ai, storage
	PurchasedAt     time.Time `bson:"purchasedAt"`
	ExpiresAt       time.Time `bson:"expiresAt"`
}

type Wallet struct {
	UserID           string       `bson:"userId"`
	LiquidBalance    int          `bson:"liquidBalance"`
	PlanBalance      int          `bson:"planBalance"`
	StorageUsedBytes int          `bson:"storageUsedBytes"` // snapshot of current used storage
	CreditPacks      []CreditPack `bson:"creditPacks,omitempty"`
	UpdatedAt        time.Time    `bson:"updatedAt"`
}

func NewWallet(userID string) *Wallet {
	return &Wallet{
		UserID:    userID,
		UpdatedAt: time.Now(),
	}
}

func (w *Wallet) Balance() int {
	return w.LiquidBalance + w.PlanBalance
}

func (w *Wallet) Deduct(amount int) error {
	if w.Balance() < amount {
		return ErrInsufficientBalance
	}
	
	remaining := amount
	
	// Deduct from PlanBalance first
	if w.PlanBalance >= remaining {
		w.PlanBalance -= remaining
		remaining = 0
	} else {
		remaining -= w.PlanBalance
		w.PlanBalance = 0
	}
	
	// Deduct the rest from LiquidBalance
	if remaining > 0 {
		w.LiquidBalance -= remaining
	}

	w.UpdatedAt = time.Now()
	return nil
}

func (w *Wallet) AddLiquid(amount int) {
	w.LiquidBalance += amount
	w.UpdatedAt = time.Now()
}

func (w *Wallet) SetPlanBalance(amount int) {
	w.PlanBalance = amount
	w.UpdatedAt = time.Now()
}

type WalletRepository interface {
	GetByUserID(ctx context.Context, userID string) (*Wallet, error)
	Update(ctx context.Context, wallet *Wallet) error
	IncrementStorage(ctx context.Context, userID string, deltaBytes int) error
}
