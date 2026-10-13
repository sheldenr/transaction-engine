package integration_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ledger/transaction-engine/internal/config"
	"github.com/ledger/transaction-engine/internal/domain"
	"github.com/ledger/transaction-engine/internal/repository/postgres"
	"github.com/ledger/transaction-engine/internal/repository/redis"
	"github.com/ledger/transaction-engine/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServices(t *testing.T) (*service.LedgerService, *postgres.DB) {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := postgres.NewDB(ctx, cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available (%v), skipping integration test", err)
		return nil, nil
	}

	if err := db.RunMigrations(ctx, postgres.SchemaMigrationSQL); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	redisClient, err := redis.NewClient(ctx, cfg)
	if err != nil {
		t.Logf("Redis not available (%v), operating without Redis", err)
		redisClient = nil
	}

	accRepo := postgres.NewAccountRepository(db)
	txnRepo := postgres.NewTransactionRepository(db)
	idempRepo := postgres.NewIdempotencyRepository(db)
	redisLock := redis.NewLockService(redisClient)

	idempService := service.NewIdempotencyService(idempRepo, redisLock, 1*time.Hour)
	ledgerService := service.NewLedgerService(accRepo, txnRepo, idempService, 1*time.Hour)

	return ledgerService, db
}

// TestHighConcurrencyDoubleSpend verifies that concurrent debits never allow the balance to drop below zero
func TestHighConcurrencyDoubleSpend(t *testing.T) {
	ledgerService, db := setupTestServices(t)
	if ledgerService == nil {
		return
	}
	defer db.Close()

	ctx := context.Background()

	// Initial setup: Alice has 10,000 cents ($100), Bob has 0 cents ($0)
	alice, err := ledgerService.CreateAccount(ctx, "Alice", "USD", 10000)
	require.NoError(t, err)
	bob, err := ledgerService.CreateAccount(ctx, "Bob", "USD", 0)
	require.NoError(t, err)

	// 50 concurrent transactions attempting to transfer 300 cents each (Total attempted: 15,000 > 10,000)
	// Exactly floor(10000 / 300) = 33 transactions should succeed, 17 should fail with insufficient funds.
	numRequests := 50
	transferAmount := int64(300)
	expectedSuccess := int64(10000 / transferAmount) // 33
	expectedFail := int64(numRequests) - expectedSuccess // 17

	var successCount int64
	var failCount int64

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			defer wg.Done()
			idempKey := fmt.Sprintf("txn-concurrency-%s-%d", uuid.New().String(), idx)
			reqPayload := []byte(fmt.Sprintf(`{"from":"%s","to":"%s","amount":%d,"idx":%d}`, alice.ID, bob.ID, transferAmount, idx))

			_, err := ledgerService.TransferFunds(
				ctx,
				alice.ID,
				bob.ID,
				transferAmount,
				fmt.Sprintf("REF-%d", idx),
				"Concurrent transfer test",
				idempKey,
				reqPayload,
			)

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, expectedSuccess, successCount, "Successful transfer count mismatch")
	assert.Equal(t, expectedFail, failCount, "Failed transfer count mismatch")

	// Verify Account Balances
	aliceFinal, err := ledgerService.GetAccount(ctx, alice.ID)
	require.NoError(t, err)
	bobFinal, err := ledgerService.GetAccount(ctx, bob.ID)
	require.NoError(t, err)

	expectedAliceBalance := int64(10000) - (expectedSuccess * transferAmount) // 100 cents = $1.00
	expectedBobBalance := expectedSuccess * transferAmount                     // 9900 cents = $99.00

	assert.Equal(t, expectedAliceBalance, aliceFinal.Balance, "Alice final balance mismatch")
	assert.Equal(t, expectedBobBalance, bobFinal.Balance, "Bob final balance mismatch")
	assert.GreaterOrEqual(t, aliceFinal.Balance, int64(0), "Account balance must NEVER drop below zero")

	// Audit Ledger Invariant
	audit, err := ledgerService.AuditLedger(ctx)
	require.NoError(t, err)
	assert.True(t, audit.IsLedgerBalanced, "Ledger total debits must equal total credits")
	assert.Empty(t, audit.UnbalancedTransactionIDs, "Found unbalanced transactions")
}

// TestBidirectionalDeadlockPrevention verifies that concurrent opposing transfers (A->B and B->A) never deadlock
func TestBidirectionalDeadlockPrevention(t *testing.T) {
	ledgerService, db := setupTestServices(t)
	if ledgerService == nil {
		return
	}
	defer db.Close()

	ctx := context.Background()

	accA, err := ledgerService.CreateAccount(ctx, "Account A", "USD", 500000)
	require.NoError(t, err)
	accB, err := ledgerService.CreateAccount(ctx, "Account B", "USD", 500000)
	require.NoError(t, err)

	numTransfers := 60
	var wg sync.WaitGroup
	wg.Add(numTransfers)

	var successCount int64

	for i := 0; i < numTransfers; i++ {
		go func(idx int) {
			defer wg.Done()
			var src, dst uuid.UUID
			if idx%2 == 0 {
				src, dst = accA.ID, accB.ID
			} else {
				src, dst = accB.ID, accA.ID
			}

			idempKey := fmt.Sprintf("deadlock-test-%s-%d", uuid.New().String(), idx)
			_, err := ledgerService.TransferFunds(
				ctx,
				src,
				dst,
				100,
				fmt.Sprintf("REF-DEADLOCK-%d", idx),
				"Bidirectional test",
				idempKey,
				[]byte(idempKey),
			)

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				t.Errorf("Transaction failed unexpectedly: %v", err)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int64(numTransfers), successCount, "All bidirectional transactions should succeed without deadlocks")
}

// TestIdempotencyRetryBehavior verifies that retrying the same transaction 5+ times executes exactly once
func TestIdempotencyRetryBehavior(t *testing.T) {
	ledgerService, db := setupTestServices(t)
	if ledgerService == nil {
		return
	}
	defer db.Close()

	ctx := context.Background()

	acc1, err := ledgerService.CreateAccount(ctx, "Sender", "USD", 100000)
	require.NoError(t, err)
	acc2, err := ledgerService.CreateAccount(ctx, "Receiver", "USD", 100000)
	require.NoError(t, err)

	idempotencyKey := "idem-key-" + uuid.New().String()
	payload := []byte(`{"sender":"test","amount":5000}`)

	// Attempt 1: Initial call
	out1, err := ledgerService.TransferFunds(
		ctx,
		acc1.ID,
		acc2.ID,
		5000,
		"TXN-IDEM-1",
		"Idempotent payment",
		idempotencyKey,
		payload,
	)
	require.NoError(t, err)
	assert.False(t, out1.IsReplay, "Initial call must not be a replay")
	initialTxnID := out1.Transaction.ID

	// Attempts 2..6: Client retries 5 times (e.g. simulated network timeouts)
	for retry := 1; retry <= 5; retry++ {
		outRetry, err := ledgerService.TransferFunds(
			ctx,
			acc1.ID,
			acc2.ID,
			5000,
			"TXN-IDEM-1",
			"Idempotent payment",
			idempotencyKey,
			payload,
		)
		require.NoError(t, err)
		assert.True(t, outRetry.IsReplay, "Subsequent calls must be recognized as idempotent replays")
		assert.Equal(t, initialTxnID, outRetry.Transaction.ID, "Replayed transaction ID must match original")
	}

	// Verify balance was only deducted ONCE
	sender, err := ledgerService.GetAccount(ctx, acc1.ID)
	require.NoError(t, err)
	receiver, err := ledgerService.GetAccount(ctx, acc2.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(95000), sender.Balance, "Sender balance must reflect exactly one deduction")
	assert.Equal(t, int64(105000), receiver.Balance, "Receiver balance must reflect exactly one credit")

	// Attempt with SAME key but DIFFERENT payload must fail with Mismatch error
	mismatchedPayload := []byte(`{"sender":"test","amount":99999}`)
	_, err = ledgerService.TransferFunds(
		ctx,
		acc1.ID,
		acc2.ID,
		5000,
		"TXN-IDEM-1",
		"Idempotent payment",
		idempotencyKey,
		mismatchedPayload,
	)
	assert.ErrorIs(t, err, domain.ErrIdempotencyKeyMismatch, "Payload mismatch must be rejected")
}
