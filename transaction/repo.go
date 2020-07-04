package transaction

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"ledger/transaction/options"
)

// Data store abstraction for querying transactions
type TransactionRepo interface {
	Create(*Transaction) error
	FindById(id string) (*Transaction, error)
	Find(opts ...*options.TransactionOptions) ([]*Transaction, error)
}

var _ TransactionRepo = (*PostgresTransactionRepo)(nil)

type PostgresTransactionRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) (*PostgresTransactionRepo, error) {
	r := &PostgresTransactionRepo{db: db}

	return r, nil
}

func (r *PostgresTransactionRepo) Create(transaction *Transaction) error {
	_, err := r.db.NamedQuery(
		`INSERT INTO transaction (sender_id, receiver_id, amount, created_at) VALUES (:sender_id, :receiver_id, 
		:amount, 
		:created_at)`,
		transaction,
	)

	return err
}

func (r *PostgresTransactionRepo) FindById(id string) (*Transaction, error) {
	var result Transaction
	err := r.db.Get(&result, "SELECT * FROM transaction WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Executes a Find operation and returns a list of Transactions
// The `transactionOptions` can be used to specify options for the operation
func (r *PostgresTransactionRepo) Find(transactionOptions ...*options.TransactionOptions) ([]*Transaction, error) {
	var result []*Transaction
	// build query
	query := "SELECT * FROM transaction"

	if len(transactionOptions) == 0 {
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
		filters["created_at"] = opt.Timestamp
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
