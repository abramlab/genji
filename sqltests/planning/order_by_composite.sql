-- setup:
CREATE TABLE test(a int, b int, c int);

CREATE INDEX test_a_b ON test(a, b);

INSERT INTO
    test (a, b, c)
VALUES
    (1, 1, 1),
    (2, 2, 2),
    (3, 3, 3),
    (4, 4, 4),
    (5, 5, 5);

-- test: non-indexed field path, ASC
EXPLAIN SELECT * FROM test ORDER BY c;
/* result:
{
    "plan": 'table.Scan("test") | docs.TempTreeSort(c)'
}
*/

-- test: non-indexed field path, DESC
EXPLAIN SELECT * FROM test ORDER BY c DESC;
/* result:
{
    "plan": 'table.Scan("test") | docs.TempTreeSortReverse(c)'
}
*/

-- test: indexed field path, ASC
EXPLAIN SELECT * FROM test ORDER BY a;
/* result:
{
    "plan": 'index.Scan("test_a_b")'
}
*/

-- test: indexed field path, DESC
EXPLAIN SELECT * FROM test ORDER BY a DESC;
/* result:
{
    "plan": 'index.ScanReverse("test_a_b")'
}
*/

-- test: indexed field path in second position, ASC
EXPLAIN SELECT * FROM test ORDER BY b;
/* result:
{
    "plan": 'table.Scan("test") | docs.TempTreeSort(b)'
}
*/

-- test: indexed field path in second position, DESC
EXPLAIN SELECT * FROM test ORDER BY b DESC;
/* result:
{
    "plan": 'table.Scan("test") | docs.TempTreeSortReverse(b)'
}
*/

-- test: filtering and sorting: >
EXPLAIN SELECT * FROM test WHERE a > 10 ORDER BY b DESC;
/* result:
{
    "plan": 'index.Scan("test_a_b", [{"min": [10], "exclusive": true}]) | docs.TempTreeSortReverse(b)'
}
*/

-- test: filtering and sorting: =
EXPLAIN SELECT * FROM test WHERE a = 10 ORDER BY b DESC;
/* result:
{
    "plan": 'index.ScanReverse("test_a_b", [{"min": [10], "exact": true}])'
}
*/


-- test: filtering and sorting with order by on first path, filter on second: =
EXPLAIN SELECT * FROM test WHERE b = 10 ORDER BY a DESC;
/* result:
{
    "plan": 'index.ScanReverse("test_a_b") | docs.Filter(b = 10)'
}
*/
