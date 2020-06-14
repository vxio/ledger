package ledger

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	host    = "localhost"
	port    = 5432
	user    = "vince"
	db_name = "transactions_dev"
)

type Repo interface {
	Create(*Transaction) error
	FindById(id string) (*Transaction, error)
	Find(opts ...TransactionOptions) ([]*Transaction, error)
}

func (r *PostgresRepo) Find(options ...TransactionOptions) ([]*Transaction, error) {
	var result []*Transaction

	var opt TransactionOptions
	if len(options) > 0 {
		opt = options[0]
	}

	// query := `SELECT * FROM transaction`

	filters := make(map[string]interface{})
	if len(opt.IDs) > 0 {
		filters["id"] = opt.IDs
	}
	filters["amount"] = []int{100}
	// filters["id"] = "62059622-316b-4e38-a5b0-c53d4312e00f"
	// "id":     "62059622-316b-4e38-a5b0-c53d4312e00f",
	// "amount": 100,

	var args []interface{}
	var where []string
	// allowedFilters := []string{"id", "amount"}
	for filter, arg := range filters { // list of allowed filters
		// pos := len(args) + 1
		var stmt string
		switch arg.(type) {
		case []string:
			stmt = fmt.Sprintf("%s in (:%s)", filter, filter)
		default:
			stmt = fmt.Sprintf("%s = :%s", filter, filter)
		}

		where = append(where, stmt)
		args = append(args, arg)
	}

	// build query
	query := "SELECT * FROM transaction"

	if len(where) > 0 {
		query = fmt.Sprintf("%s WHERE %s",
			query,
			strings.Join(where, " AND "),
		)
	}

	query, args, err := sqlx.Named(query, filters)
	query, args, err = sqlx.In(query, args...)
	query = r.db.Rebind(query)
	err = r.db.Select(&result, query, args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}

var _ Repo = (*PostgresRepo)(nil)

type PostgresRepo struct {
	db *sqlx.DB
}

func GetPostgresDB() (*sqlx.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable",
		host,
		port,
		user,
		db_name,
	)

	db, err := sqlx.Connect("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %v", err)
	}

	return db, nil
}

func NewPostgresRepo(db *sqlx.DB) (*PostgresRepo, error) {
	return &PostgresRepo{db: db}, nil
}

func (r *PostgresRepo) Init() error {
	_, err := r.db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepo) Create(v *Transaction) error {
	_, err := r.db.NamedQuery(
		`INSERT INTO transaction (from_id, to_id, amount, timestamp) VALUES (:from_id, :to_id, :amount, :timestamp)`,
		v,
	)

	return err
}

func (r *PostgresRepo) FindById(id string) (*Transaction, error) {
	var result Transaction
	err := r.db.Get(&result, "SELECT * FROM transaction WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *PostgresRepo) createTables() error {
	var err error

	var schema = `
	CREATE TABLE IF NOT EXISTS transaction (
	id uuid DEFAULT uuid_generate_v4(),
	from_id uuid NOT NULL,
	to_id uuid NOT NULL,
	amount integer NOT NULL,
	timestamp timestamp,
	
	 PRIMARY KEY (id)
	                         )
	`
	_, err = r.db.Exec(schema)
	if err != nil {
		return err
	}
	return nil
}

type Transaction struct {
	ID        string     `db:"id"` // unique id
	FromID    string     `db:"from_id"`
	ToID      string     `db:"to_id"`
	Amount    int        `db:"amount"`
	Timestamp *time.Time `db:"timestamp"`
}
