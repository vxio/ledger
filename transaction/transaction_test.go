package transaction

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"ledger/transaction/options"
	"ledger/transaction/postgres"
)

func TestTransactionRepo(t *testing.T) {
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
	repo                TransactionRepo
	db                  *sqlx.DB
	transactions        []*Transaction
}

func (s *Suite) SetupSuite() {
	// load environment
	err := godotenv.Load("../.env")
	s.NoError(err)

	config, err := postgres.Parse()
	s.NoError(err)

	db, err := postgres.Connect(config)
	s.NoError(err)
	s.db = db

	repo, err := NewPostgresRepo(db)
	s.NoError(err)

	s.repo = repo
}

func (s *Suite) SetupTest() {
	s.teardown()
	s.createTransactions(10)
}

func (s *Suite) teardown() {
	s.db.MustExec("DElETE FROM transaction")
}

func (s *Suite) createTransactions(length int) {
	var rows []*Transaction
	for i := 1; i <= length; i++ {
		rows = append(rows,
			&Transaction{
				SenderID:   uuid.New(),
				ReceiverID: uuid.New(),
				Amount:     decimal.NewFromInt32(int32(i * 100)),
			},
		)
	}

	query := "INSERT INTO transaction (sender_id, receiver_id, amount) VALUES (:sender_id, :receiver_id, :amount)"
	_, err := s.db.NamedExec(query, rows)
	s.NoError(err)

	s.refreshInMem()
}

func (s *Suite) TeardownSuite() {
	s.NoError(s.db.Close())
}

func (s *Suite) TestFindById() {
	want := s.transactions[1]
	got, err := s.repo.FindById(want.ID)
	s.NoError(err)

	s.Equal(want, got)
}

func (s *Suite) TestFindAll() {
	got, err := s.repo.Find()
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

	transactions, err := s.repo.Find(opts)
	s.NoError(err)

	s.Equal(s.transactions[:num], transactions)
}

func (s *Suite) TestFindByAmountRange() {
	cases := []struct {
		From int
		To   int
	}{
		{200, 800},
		{0, 300},
		{400, math.MaxInt32},
	}
	for _, tc := range cases {
		from := decimal.NewFromInt32(int32(tc.From))
		to := decimal.NewFromInt32(int32(tc.To))

		intRange := &options.DecimalRange{
			Low:  &from,
			High: &to,
		}

		opts := options.NewTransactionOptions()
		opts.SetAmountRange(intRange)
		got, err := s.repo.Find(opts)
		s.NoError(err)

		var want []*Transaction
		for _, each := range s.transactions {
			if !from.IsZero() && each.Amount.LessThan(from) {
				continue
			}
			if !to.IsZero() && each.Amount.GreaterThan(to) {
				continue
			}

			want = append(want, each)
		}

		s.Equal(want, got, "values should range from %s to %s", from.String(), to.String())
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
		query := "UPDATE transaction SET created_at =  now() WHERE id = $1"
		s.db.MustExec(query, s.transactions[0].ID)
		s.refreshInMem()

		timeRange := &options.TimeRange{
			Low:  tc.From,
			High: tc.To,
		}

		opts := options.NewTransactionOptions()
		opts.SetTimeRange(timeRange)
		got, err := s.repo.Find(opts)
		s.NoError(err)

		var want []*Transaction
		for _, each := range s.transactions {
			if !each.CreatedAt.IsZero() &&
				tc.From != nil && each.CreatedAt.Sub(*tc.From) < 0 &&
				tc.To != nil && each.CreatedAt.Sub(*tc.To) > 0 {

				want = append(want, each)
			}
		}

		s.Equal(got, want)
	}

}

func Time(v time.Time) *time.Time {
	return &v
}

func (s *Suite) refreshInMem() {
	query := "SELECT * FROM transaction"
	s.transactions = s.transactions[:0] // clear our in-memory transactions
	err := s.db.Select(&s.transactions, query)
	s.NoError(err)
}
