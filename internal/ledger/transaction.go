package ledger

import (
	"fmt"
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
	Get(id string) (*Transaction, error)
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

func (r *PostgresRepo) Get(id string) (*Transaction, error) {
	var result Transaction
	err := r.db.Get(result, "SELECT * from transaction WHERE id = ?", id)
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
	ID        string    `db:"id"` // unique id
	FromID    string    `db:"from_id"`
	ToID      string    `db:"to_id"`
	Amount    int       `db:"amount"`
	Timestamp time.Time `db:"timestamp"`
}
