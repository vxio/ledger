package ledger

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"proglog/internal/ledger/options"
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
	Find(opts ...*options.TransactionOptions) ([]*Transaction, error)
}

func (r *PostgresRepo) Find(transactionOptions ...*options.TransactionOptions) ([]*Transaction, error) {
	var result []*Transaction
	// build query
	query := "SELECT * FROM transaction"

	if len(transactionOptions) == 0 { // return all
		err := r.db.Select(&result, query)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	opt := transactionOptions[0]

	filters := make(map[string]interface{})
	if len(opt.IDs) > 0 {
		filters["id"] = opt.IDs
	}
	if opt.Amount != nil {
		filters["amount"] = opt.Amount
	}

	if opt.Timestamp != nil {
		filters["timestamp"] = opt.Timestamp
	}

	var where []string
	var args []interface{}
	namedParams := make(map[string]interface{})

	updateQueryParams := func(stmt, key string, value interface{}) {
		where = append(where, stmt)
		args = append(args, value)
		namedParams[key] = value
	}

	for columnName, arg := range filters {
		switch v := arg.(type) {
		case options.Range:
			var key string

			from, ok := v.From()
			if ok {
				key = columnName + "_from"
				fromStmt := fmt.Sprintf("%s >= :%s", columnName, key)
				updateQueryParams(fromStmt, key, from)
			}
			to, ok := v.To()
			if ok {
				key = columnName + "_to"
				toStmt := fmt.Sprintf("%s <= :%s", columnName, key)
				updateQueryParams(toStmt, key, to)
			}

		default:
			stmt := fmt.Sprintf("%s in (:%s)", columnName, columnName)
			updateQueryParams(stmt, columnName, v)
		}
	}

	if len(where) > 0 {
		query = fmt.Sprintf("%s WHERE %s",
			query,
			strings.Join(where, " AND "),
		)
	}

	query, args, err := sqlx.Named(query, namedParams)
	if err != nil {
		return nil, err
	}
	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}
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

	// set default timezone to utc
	r.db.MustExec("SET timezone to 'UTC'")

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
