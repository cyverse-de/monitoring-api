package checkresults

import (
	"context"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type CheckResult struct {
	Node         string                 `db:"node"`
	Error        string                 `db:"error"`
	Successful   bool                   `db:"successful"`
	Result       map[string]interface{} `db:"result"`
	CheckType    string                 `db:"check_type"`
	DateSent     time.Time              `db:"date_sent"`
	DateReceived time.Time              `db:"date_received"`
}

type CheckResulter struct {
	dbconn *sqlx.DB
}

func New(dbconn *sqlx.DB) *CheckResulter {
	return &CheckResulter{dbconn: dbconn}
}

func (c *CheckResulter) GetCheckResults(ctx context.Context, limit, offset uint) ([]*CheckResult, error) {
	checkResultsT := goqu.T("check_results")
	query := goqu.From(checkResultsT).
		Select(
			checkResultsT.Col("node"),
			checkResultsT.Col("error"),
			checkResultsT.Col("successful"),
			checkResultsT.Col("check_type"),
			checkResultsT.Col("date_sent"),
			checkResultsT.Col("date_received"),
			checkResultsT.Col("result"),
		).
		Order(checkResultsT.Col("date_received").Desc()).
		Limit(limit).
		Offset(offset)

	queryString, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := c.dbconn.QueryxContext(ctx, queryString, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*CheckResult, 0)
	for rows.Next() {
		var r CheckResult
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
