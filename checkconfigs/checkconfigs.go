package checkconfigs

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type CheckConfiguration struct {
	ID            string `db:"id"`
	CheckType     string `db:"check_type"`
	Configuration string `db:"configuration"`
	FormatVersion int    `db:"format_version"`
}

type QuerySettings struct {
	hasLimit  bool
	limit     uint
	hasOffset bool
	offset    uint
}

type QueryOption func(*QuerySettings)

func WithQueryLimit(limit uint) QueryOption {
	return func(s *QuerySettings) {
		s.hasLimit = true
		s.limit = limit
	}
}

func WithQueryOffset(offset uint) QueryOption {
	return func(s *QuerySettings) {
		s.hasOffset = true
		s.offset = offset
	}
}

type CheckConfigurator struct {
	dbconn *sqlx.DB
}

func New(dbconn *sqlx.DB) *CheckConfigurator {
	return &CheckConfigurator{dbconn: dbconn}
}

func (a *CheckConfigurator) GetCheckConfigurations(ctx context.Context, opts ...QueryOption) ([]*CheckConfiguration, error) {
	querySettings := &QuerySettings{}
	for _, opt := range opts {
		opt(querySettings)
	}

	cfgT := goqu.T("check_configurations")
	query := goqu.From(cfgT).
		Select(
			cfgT.Col("id"),
			cfgT.Col("check_type"),
			cfgT.Col("configuration"),
			cfgT.Col("format_version"),
		)

	if querySettings.hasLimit {
		query = query.Limit(querySettings.limit)
	}

	if querySettings.hasOffset {
		query = query.Offset(querySettings.offset)
	}

	queryString, _, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := a.dbconn.QueryxContext(ctx, queryString)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*CheckConfiguration, 0)
	for rows.Next() {
		var r CheckConfiguration
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
