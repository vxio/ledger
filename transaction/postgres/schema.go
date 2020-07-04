package postgres

import "github.com/jmoiron/sqlx"

func createTransactionsTable(db *sqlx.DB) error {
	var schema = `
	CREATE TABLE IF NOT EXISTS transaction (
	id uuid DEFAULT uuid_generate_v4() PRIMARY KEY ,
	sender_id uuid NOT NULL,
	receiver_id uuid NOT NULL,
	amount  NUMERIC(12, 4) DEFAULT 0 NOT NULL,
	created_at timestamp DEFAULT now()
	                         )
	`
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}
	return nil
}
