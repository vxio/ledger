package ledger

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"proglog/internal/ledger/options"
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
	s.createTransactions(10)
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
	var ids []string
	num := 2

	for i := 0; i < num; i++ {
		ids = append(ids, s.transactions[i].ID)
	}

	opts := options.NewTransactionOptions()
	opts.SetIDs(ids...)

	transactions, err := s.Repo.Find(opts)
	s.NoError(err)

	s.Equal(s.transactions[:num], transactions)
}

func (s *Suite) TestFindByAmountRange() {
	cases := []struct {
		From *int
		To   *int
	}{
		{Int(200), Int(800)},
		{nil, Int(300)},
		{Int(200), nil},
	}
	for _, tc := range cases {
		intRange := &options.IntRange{
			Low:  tc.From,
			High: tc.To,
		}

		opts := options.NewTransactionOptions()
		opts.SetAmountRange(intRange)
		got, err := s.Repo.Find(opts)
		s.NoError(err)

		var want []*Transaction
		for _, each := range s.transactions {
			if tc.From != nil && each.Amount < *tc.From {
				continue
			}
			if tc.To != nil && each.Amount > *tc.To {
				continue
			}

			want = append(want, each)
		}

		s.Equal(want, got)
	}
}

func (s *Suite) TestFindByTimeRange() {
	now := time.Now()
	cases := []struct {
		From      *time.Time
		To        *time.Time
		Timestamp time.Time // for one transaction in our test
	}{
		{Time(now.AddDate(0, -1, 0)), Time(now.Add(time.Hour)), now},
		{Time(now.AddDate(0, 0, 1)), nil, now},
	}
	for _, tc := range cases {
		// update one
		query := "UPDATE transaction SET timestamp =  now() WHERE id = $1"
		s.db.MustExec(query, s.transactions[0].ID)
		// update in-memory
		query = "SELECT * FROM transaction"
		s.transactions = s.transactions[:0] // clear our in-memory transactions
		err := s.db.Select(&s.transactions, query)
		s.NoError(err)

		timeRange := &options.TimeRange{
			Low:  tc.From,
			High: tc.To,
		}

		opts := options.NewTransactionOptions()
		opts.SetTimeRange(timeRange)
		got, err := s.Repo.Find(opts)
		s.NoError(err)

		var want []*Transaction
		for _, each := range s.transactions {
			if each.Timestamp != nil &&
				tc.From != nil && each.Timestamp.Sub(*tc.From) < 0 &&
				tc.To != nil && each.Timestamp.Sub(*tc.To) > 0 {

				want = append(want, each)
			}
		}

		s.Equal(got, want)
	}

}

func Int(v int) *int {
	return &v
}

func Time(v time.Time) *time.Time {
	return &v
}
