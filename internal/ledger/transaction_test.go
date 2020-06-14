package ledger

import (
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"proglog/testutil"
)

func TestSome(t *testing.T) {
	db, err := GetPostgresDB()
	require.NoError(t, err)
	defer db.Close()

	repo, err := NewPostgresRepo(db)
	require.NoError(t, err)

	db.MustExec("DROP TABLE IF EXISTS transaction")
	require.NoError(t, repo.createTables())

	transaction := &Transaction{
		FromID: testutil.GenerateUUID(),
		ToID:   testutil.GenerateUUID(),
		Amount: 100,
	}

	err = repo.Create(transaction)
	require.NoError(t, err)
}
