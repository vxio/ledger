package ledger

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"proglog/testutil"
)

func Test(t *testing.T) {
	s := NewSuite(t)
	suite.Run(t, s)
}

func NewSuite(t *testing.T) *Suite {
	return &Suite{
		Assertions: require.New(t),
	}
}

type Suite struct {
	suite.Suite
	*require.Assertions // default to require behavior
	Repo                Repo
	db                  *sqlx.DB
	transactions        []*Transaction
}

func (s *Suite) SetupSuite() {
	db, err := GetPostgresDB()
	s.NoError(err)
	s.db = db

	repo, err := NewPostgresRepo(db)
	s.NoError(err)
	s.Repo = repo

	s.NoError(repo.createTables())
}

func (s *Suite) SetupTest() {
	s.db.MustExec("DElETE FROM transaction")
	s.createTransactions(5)
}

func (s *Suite) createTransactions(length int) {
	var rows []*Transaction
	for i := 1; i <= length; i++ {
		rows = append(rows,
			&Transaction{
				FromID: testutil.NewUUID(),
				ToID:   testutil.NewUUID(),
				Amount: i * 100,
			},
		)
	}

	query := "INSERT INTO transaction (from_id, to_id, amount) VALUES (:from_id, :to_id, :amount)"
	_, err := s.db.NamedExec(query, rows)
	s.NoError(err)

	query = "SELECT * FROM transaction"

	s.transactions = s.transactions[:0] // clear our in-memory transactions
	err = s.db.Select(&s.transactions, query)
	s.NoError(err)
}

func (s *Suite) TeardownSuite() {
	s.NoError(s.db.Close())
}

func (s *Suite) TestFindById() {
	want := s.transactions[1]
	got, err := s.Repo.FindById(want.ID)
	s.NoError(err)

	s.Equal(want, got)
}

func (s *Suite) TestFindAll() {
	got, err := s.Repo.Find()
	s.NoError(err)

	s.Equal(got, s.transactions)
}

func (s *Suite) TestFindByIds() {

}

// func TestSome(t *testing.T) {
// 	transaction := &Transaction{
// 		FromID: testutil.NewUUID(),
// 		ToID:   testutil.NewUUID(),
// 		Amount: 100,
// 	}
//
// 	err = repo.Create(transaction)
// 	require.NoError(t, err)
// }
//
// func TestFind(t *testing.T)
