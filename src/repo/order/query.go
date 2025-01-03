package order_repo

import (
	"TestTaskJustPay/src/domain"
	"fmt"
	"github.com/elliotchance/pie/v2"
	"github.com/jackc/pgx/v5"
	"strings"
)

const getEventsQuery = `
SELECT id, order_id, user_id, status, created_at, updated_at
FROM event 
WHERE order_id = $1
ORDER BY received_at ASC
`

func filterOrdersArgs(filter domain.Filter) pgx.NamedArgs {
	status := pie.Map(filter.Status, func(v domain.OrderStatus) string { return string(v) })
	args := pgx.NamedArgs{
		"user_id": filter.UserID,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	}
	// TODO: abstract to pkg
	for _, m := range []struct {
		values []string
		prefix string
	}{{status, "status"}} {
		for i, v := range genMap(m.values, genIndexes(m.values, m.prefix)) {
			args[i] = v
		}
	}

	return args
}

func filterOrdersQuery(filter domain.Filter) string {
	orderStr := fmt.Sprintf(" ORDER BY %s %s", filter.SortBy, filter.SortOrder)
	query := fmt.Sprintf(`
SELECT 
	id, user_id, status, created_at, updated_at 
FROM "order"
WHERE
	user_id = @user_id
	AND status IN ( %v )
%v
LIMIT @limit OFFSET @offset;
`, strings.Join(genIndexes(filter.Status, "@status"), ","),
		orderStr)

	return query
}

// TODO: abstract to pkg
func genIndexes[T any](values []T, prefix string) []string {
	res := make([]string, len(values))
	for i := range values {
		res[i] = fmt.Sprintf("%v_%d", prefix, i)
	}
	return res
}

// TODO: abstract to pkg
func genMap[V any](values []V, keys []string) map[string]V {
	res := make(map[string]V, len(values))
	for i := range values {
		res[keys[i]] = values[i]
	}
	return res
}
