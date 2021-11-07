module github.com/genjidb/genji/engine/badgerengine

go 1.16

require (
	github.com/dgraph-io/badger/v3 v3.2103.2
	github.com/genjidb/genji v0.14.0
	github.com/stretchr/testify v1.7.0
)

replace github.com/genjidb/genji v0.14.0 => ../../
