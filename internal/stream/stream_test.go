package stream_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/genjidb/genji/internal/environment"
	"github.com/genjidb/genji/internal/sql/parser"
	"github.com/genjidb/genji/internal/stream"
	"github.com/genjidb/genji/internal/testutil"
	"github.com/genjidb/genji/internal/testutil/assert"
	"github.com/stretchr/testify/require"
)

func TestStream(t *testing.T) {
	s := stream.New(stream.Documents(
		testutil.MakeDocument(t, `{"a": 1}`),
		testutil.MakeDocument(t, `{"a": 2}`),
	))

	s = s.Pipe(stream.Map(parser.MustParseExpr("{a: a + 1}")))
	s = s.Pipe(stream.Filter(parser.MustParseExpr("a > 2")))

	var count int64
	err := s.Iterate(new(environment.Environment), func(env *environment.Environment) error {
		d, ok := env.GetDocument()
		require.True(t, ok)
		tt, err := json.Marshal(d)
		require.NoError(t, err)
		require.JSONEq(t, fmt.Sprintf(`{"a": %d}`, count+3), string(tt))
		count++
		return nil
	})
	assert.NoError(t, err)
	require.Equal(t, int64(1), count)
}
