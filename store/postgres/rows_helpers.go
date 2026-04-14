package postgres

import "github.com/jackc/pgx/v5"

func decodeDocRows[T any](rows pgx.Rows) ([]T, error) {
	out := []T{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item T
		if err := decodeJSON(raw, &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
