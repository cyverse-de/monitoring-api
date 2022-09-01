package checktypes

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type CheckType struct {
	ID          string `db:"id"`
	Name        string `db:"name"`
	Description string `db:"description"`
}

type CheckTyper struct {
	dbconn *sqlx.DB
}

func (c *CheckTyper) GetCheckTypes(ctx context.Context) ([]*CheckType, error) {
	typesT := goqu.T("check_types")
	query := goqu.From(typesT).
		Select(
			typesT.Col("id"),
			typesT.Col("name"),
			typesT.Col("description"),
		)
	queryString, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := c.dbconn.QueryxContext(ctx, queryString)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*CheckType, 0)
	for rows.Next() {
		var r CheckType
		err = rows.StructScan(&r)
		if err != nil {
			return nil, err
		}
		results = append(results, &r)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return results, nil
}
