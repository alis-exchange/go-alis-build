# Alis Build ProtoDB Package (`protodb`)

`protodb` is a library of interfaces, defaults, and building blocks for resource-oriented
database tables — it does not ship a table implementation. The table body (the concrete type
that implements `protodb.ResourceTable[R]` against a specific schema) lives with the consumer,
including generated blocks; the library's job is to make that body a thin, readable composition
of shared parts rather than pasted plumbing.

```go
import "go.alis.build/protodb/v2"
```

## Contents

- [Core types](#core-types)
- [Errors](#errors)
- [Reading: List, Stream, ListOptions/StreamOptions](#reading-list-stream-listoptionsstreamoptions)
- [Filtering](#filtering)
- [Ordering](#ordering)
- [Transactions](#transactions)
- [`spanneradapter`: building blocks for a Spanner-backed table](#spanneradapter-building-blocks-for-a-spanner-backed-table)
- [`memadapter` and `protodbtest`: testing](#memadapter-and-protodbtest-testing)
- [Migrating from v2.0.x](#migrating-from-v20x)

## Core types

### `ResourceTable[R]`

The single generic table interface. There is no proto-constrained variant — `R` may be any
type.

```go
type ResourceTable[R any] interface {
	Create(ctx context.Context, rows ...*Row[R]) error
	Write(ctx context.Context, rows ...*Row[R]) error
	Read(ctx context.Context, key Key) (*Row[R], error)
	BatchRead(ctx context.Context, keys ...Key) ([]*Row[R], error)
	List(ctx context.Context, opts ListOptions) (rows []*Row[R], nextPageToken string, err error)
	Stream(ctx context.Context, opts StreamOptions) iter.Seq2[*Row[R], error]
	Delete(ctx context.Context, keys ...Key) error
	WritePolicies(ctx context.Context, entries ...PolicyEntry) error
}
```

`Create`, `Write`, `Delete`, and `WritePolicies` are all variadic and all no-op on a zero-length
call — `table.Delete(ctx)` returns `nil` without touching storage. This lets a caller collect
zero or more rows into a slice and always spread it into the call, no `if len(x) > 0` guard
needed.

### `Row[R]` and `Key`

`Row[R]` is pure data — it carries no methods and no reference back to the table. To persist a
change, mutate the `Row` you have and call `Write` with it:

```go
row, err := table.Read(ctx, key)
if err != nil {
	return err
}
row.Resource = updated
err = table.Write(ctx, row)
```

```go
type Row[R any] struct {
	Key      Key
	Resource R
	Policy   *iampb.Policy // nil means "no policy", not "empty policy"
}
```

`Key` is database-agnostic — no Spanner or SQL types leak into it:

```go
type Key interface {
	KeyValues() []any
}
```

Adapter packages provide concrete `Key` implementations (`spanneradapter.StringKey`, or a
tagged struct built with `spanneradapter.KeySpecFor`, below).

### `PolicyEntry`

Pairs a `Key` with the IAM policy to write for it, for `WritePolicies`:

```go
type PolicyEntry struct {
	Key    Key
	Policy *iampb.Policy
}
```

Policy is per-row, not a table-wide feature: a table implementation with no policy column
simply configures it away (`spanneradapter.Scanner.PolicyColumn = ""`; see below) and
`WritePolicies` becomes unreachable for that table's rows in practice.

## Errors

| Operation | Condition | Result |
|---|---|---|
| `Read` | key does not exist | `NotFound` status error |
| `BatchRead` | some keys do not exist | those slots are `nil` in the returned slice — **not** an error |
| `Create` | any key already exists | `AlreadyExists` status error |
| `Delete` | key does not exist | `nil` (idempotent — matches Spanner mutation semantics) |
| `List` | `PageSize < 0` | `InvalidArgument` status error |
| any op | malformed input (bad filter, bad order-by, bad page token) | `InvalidArgument` status error |

```go
row, err := table.Read(ctx, key)
if protodb.IsNotFound(err) {
	// handle absence
}
```

```go
func IsNotFound(err error) bool
func IsAlreadyExists(err error) bool
```

Both return `false` for a `nil` error and for any other status code; they check `status.Code(err)`.

## Reading: List, Stream, ListOptions/StreamOptions

`List` is always bounded; `Stream` always traverses everything. There is no `Query` — anything
that used to reach for `Query` now reaches for `List`.

### `ListOptions`

```go
type ListOptions struct {
	Parent       string
	PageSize     int32
	PageToken    string
	Filter       string
	FilterParams map[string]any
	OrderBy      string
	Tail         bool
}
```

- **`PageSize`**: `0` means `protodb.DefaultPageSize` (100). Negative returns `InvalidArgument`.
  `List` never returns more than `PageSize` rows in one call — there is no "unbounded List";
  use `Stream` for that.
- **`Tail`**: when `true`, returns the *last* `PageSize` rows of the given order, still returned
  in that order (not reversed). Useful for "give me the most recent N" without the caller
  having to invert `OrderBy` and reverse the result by hand.
- **`PageToken`**: opaque and fingerprinted. A token minted by one `(Parent, Filter, OrderBy,
  Tail)` combination is only valid against a follow-up call with that exact same combination —
  presenting it against a different query (even just a different `Filter`) returns
  `InvalidArgument`. Never construct or inspect a token; only pass one back verbatim from a
  prior response.

```go
rows, nextPageToken, err := table.List(ctx, protodb.ListOptions{
	Parent:   "publishers/acme/books/",
	PageSize: 50,
	OrderBy:  "create_time desc",
})
```

### `StreamOptions` and `Stream`

```go
type StreamOptions struct {
	Parent       string
	Filter       string
	FilterParams map[string]any
	OrderBy      string
}
```

No `PageSize`, no `PageToken`, no `Tail` — a stream always traverses every matching row, in
`OrderBy` order. `Stream` returns an `iter.Seq2[*Row[R], error]`, consumed with a range-over-func
loop:

```go
for row, err := range table.Stream(ctx, protodb.StreamOptions{Parent: "publishers/acme/"}) {
	if err != nil {
		return err
	}
	// process row
}
```

Errors are yielded as the final element and end the sequence. `break` (or an early `return`)
cancels the traversal cleanly — the implementation must not leak goroutines or hold a
transaction open past that point.

## Filtering

`ListOptions.Filter` / `StreamOptions.Filter` are [AIP-160](https://google.aip.dev/160) filter
expressions, parsed by the `filtering` subpackage (absorbed from `sproto`, whose copies remain for
now and will be deprecated in favor of this package later).

Untrusted values enter a filter through `param('name')` plus `FilterParams` — **never** by
splicing values into the filter string with `fmt.Sprintf` or string concatenation:

```go
// Right: value is bound as a query parameter, never interpolated into SQL text.
rows, _, err := table.List(ctx, protodb.ListOptions{
	Filter:       "app_name == param('app') AND user_id == param('user')",
	FilterParams: map[string]any{"app": appName, "user": userID},
})

// Wrong — DO NOT DO THIS: string-splicing a caller-controlled value into a
// filter expression is a SQL/CEL injection vector.
// Filter: fmt.Sprintf("app_name == '%s'", appName)
```

`param('name')` resolves against `FilterParams["name"]`; referencing an unknown name, or using
`param(...)` with no `FilterParams` supplied, is a parse error. This holds even inside
`like()`/`prefix()`/`suffix()` calls — the bound placeholder is embedded directly rather than
re-wrapped, so `like(name, param('p'))` binds exactly one parameter.

To match rows where a field is unset, compare it with `NULL`: `Book.delete_time = NULL` compiles to
`... IS NULL` and `!= NULL` to `IS NOT NULL`, keeping the identifier's rewrite (for example a
`filtering.Timestamp` field). `<`, `<=`, `>` and `>=` against `NULL` are rejected as invalid
filters. Before v2.2.0, `= NULL` compared against the string `"NULL"` and never matched an unset
value.

`Parse` backtick-quotes the column each path starts from (`` `Book`.delete_time ``), so a column
named after a GoogleSQL keyword, such as a resource column called `Set`, `Order` or `Group`,
filters like any other. `filtering.Reserved` is no longer needed.

`Parser.Compile` evaluates the same filters in memory (`memadapter` uses it), with SQL's NULL
rules. Where `Parse` binds something other than the value itself, such as an identifier on the
right of a comparison (bound as the string `'b'` in `a = b`), `Compile` returns `Unimplemented`
rather than giving a different answer from Spanner.

## Ordering

`OrderBy` is an [AIP-132](https://google.aip.dev/132) order-by expression, parsed by the
`ordering` subpackage (also absorbed unchanged from `sproto`):

```go
order, err := ordering.NewOrder("age desc, name asc")
cols := order.Columns() // []ordering.ColumnOrder{{Column: "age", Desc: true}, {Column: "name", Desc: false}}
```

Ordering is always deterministic: whatever the caller's `OrderBy` doesn't already mention, the
table's key columns are appended as a tiebreaker (ascending, or descending under `Tail`), so
rows with equal order values still get one stable position. That determinism is what makes a
keyset page token correct — see `spanneradapter.StatementBuilder` below.

## Transactions

### `TransactionRunner`

```go
type TransactionRunner interface {
	RunTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Implementations (e.g. `spanneradapter.SpannerTransactionRunner`) inject the transaction into the
`ctx` passed to `fn`. Any `ResourceTable` operation that receives that `ctx` — on *any* table
sharing the same underlying database client — joins the same transaction, which commits if `fn`
returns `nil` and rolls back otherwise.

Contract, worth internalizing before writing `fn`:

- **`fn` may run more than once.** Spanner aborts and retries contended transactions; `fn` must
  be re-runnable — mutate only through the `ctx` it is given, never an external side effect.
- **No read-your-writes.** Writes are buffered client-side until commit; a read inside `fn`
  observes the database as of transaction start, not `fn`'s own buffered writes so far.
- **Cross-table**: any table operation that receives `fn`'s `ctx` participates, so a single
  transaction can span multiple tables.

```go
txRunner := &spanneradapter.SpannerTransactionRunner{Client: spannerClient}
err := txRunner.RunTransaction(ctx, func(ctx context.Context) error {
	row, err := tableA.Read(ctx, keyA)
	if err != nil {
		return err
	}
	if err := tableB.Write(ctx, &protodb.Row[B]{Key: keyB, Resource: newB}); err != nil {
		return err
	}
	row.Resource = updatedA
	return tableA.Write(ctx, row)
})
```

### `ReadModifyWrite`

A read-check-mutate-write helper built on `TransactionRunner`: an atomic compare-and-swap under
Spanner's serializable isolation.

```go
func ReadModifyWrite[R any](ctx context.Context, runner TransactionRunner, table ResourceTable[R], key Key, fn func(*Row[R]) error) error
```

`fn` inherits `RunTransaction`'s "may run more than once" contract — mutate only the `*Row[R]`
you're given:

```go
err := protodb.ReadModifyWrite(ctx, txRunner, table, key, func(row *protodb.Row[Book]) error {
	row.Resource.Stock -= 1
	if row.Resource.Stock < 0 {
		return status.Error(codes.FailedPrecondition, "out of stock")
	}
	return nil
})
```

## `spanneradapter`: building blocks for a Spanner-backed table

`spanneradapter` does not ship a `ResourceTable` implementation. It ships the pieces a
consumer-owned Spanner table composes, so that composition is a handful of struct literals
rather than hand-rolled scan/query/paging logic repeated at every call site.

### `KeySpec`

Declares a table's key structure: which columns compose it, how to scope a query to a parent
resource, how to decode a key out of a Spanner row, and how to encode a key to a canonical
in-process string (for joining `BatchRead` results back to requested keys — never decoded back
into a key).

```go
type KeySpec interface {
	Columns() []string
	ParentFilter(parent string) (sql string, params map[string]any)
	Decode(row *spanner.Row) (protodb.Key, error)
	Encode(key protodb.Key) (string, error)
}
```

**`StringKeySpec`** covers the common single string-column case, with parent scoping as a
prefix match (`STARTS_WITH`) — the natural fit for resource-name-shaped keys:

```go
spec := spanneradapter.StringKeySpec("key") // e.g. "publishers/acme/books/1"
```

**`KeySpecFor`** builds a `KeySpec` for a multi-column key by reflecting once over a
`pdb`-tagged struct. Every exported field must carry a `pdb:"column_name"` tag and be one of
`string`, `int64`, `bool`, `float64`, or `time.Time`:

```go
type sessionKey struct {
	SessionID string `pdb:"session_id"`
	AppName   string `pdb:"app_name"`
	UserID    string `pdb:"user_id"`
}

func (k sessionKey) KeyValues() []any { return spanneradapter.KeyValuesOf(k) }

spec, err := spanneradapter.KeySpecFor[sessionKey]()
```

`spanneradapter.KeyValuesOf(k)` is the reflection helper behind that `KeyValues()`
implementation — it reads the same `pdb`-tagged, exported fields in field order. Multi-column
parent scoping is not implemented in this version: `KeySpecFor` accepts only the default
(`WithParentColumns(1)`, matching against the first field) and rejects any other value at
construction.

### `ValueCodec`

Declares how a resource type is stored in and read from its column:

```go
type ValueCodec[R any] interface {
	NullDest() any
	Value(dest any) (val R, valid bool, err error)
}
```

Shipped codecs: `ProtoCodec[R proto.Message]()` for protobuf resources, `Int64Codec()`, and
`StringCodec()`.

### `Scanner[R]`

Composes a `KeySpec` and a `ValueCodec` (plus an optional policy column) into one `ScanRow`
operation:

```go
type Scanner[R any] struct {
	Spec           KeySpec
	Codec          ValueCodec[R]
	ResourceColumn string
	PolicyColumn   string // "" means the table has no policy column
}

func (s Scanner[R]) Columns() []string
func (s Scanner[R]) ScanRow(row *spanner.Row) (*protodb.Row[R], error)
```

`Columns()` is the exact projection `ScanRow` needs — the same set a `StatementBuilder` SELECT
must produce (see `TestBuildListProjectsExactlyWhatScannerNeeds` in the test suite for the
invariant this protects).

### Executor: `Apply`, `Query`, `ReadRowByKey`, `ReadKeySet`

Four free functions that check the `ctx` for an active transaction
(`spanneradapter.SpannerTxFromContext`) and route accordingly — a transaction's buffered
mutations/reads, or a single-use snapshot/apply against the client directly:

```go
func Apply(ctx context.Context, client *spanner.Client, ms []*spanner.Mutation) error
func Query(ctx context.Context, client *spanner.Client, stmt spanner.Statement) *spanner.RowIterator
func ReadRowByKey(ctx context.Context, client *spanner.Client, table string, key spanner.Key, columns []string) (*spanner.Row, error)
func ReadKeySet(ctx context.Context, client *spanner.Client, table string, keys spanner.KeySet, columns []string) *spanner.RowIterator
```

`Apply` and `ReadRowByKey` convert errors with `ErrorToStatus`. `Query` and `ReadKeySet` hand
back the raw `*spanner.RowIterator` — callers own `Stop()` and error mapping on iteration.

### `StatementBuilder`

Assembles the SELECT statements behind `List` and `Stream`: projection, parent scoping, AIP-160
filter, keyset cursor, deterministic `ORDER BY`, and the paging limit.

```go
type StatementBuilder struct {
	TableName      string
	Spec           KeySpec
	ResourceColumn string
	PolicyColumn   string
	Parser         *filtering.Parser // required whenever a filter is supplied
}

func (b StatementBuilder) BuildList(opts protodb.ListOptions) (spanner.Statement, []ordering.ColumnOrder, uint64, error)
func (b StatementBuilder) BuildStream(opts protodb.StreamOptions) (spanner.Statement, error)
```

`BuildList` returns `(stmt, order, fingerprint, err)`. `order` is the query's **effective
order** — the caller's `OrderBy`, inverted under `Tail`, with every key column not already
mentioned appended as a tiebreaker. `fingerprint` is computed over the request's original
(non-inverted) `Parent`/`Filter`/`OrderBy`/`Tail`, and is what `EncodePageToken`/
`DecodePageToken` bind a token to.

The statement's `LIMIT` is `PageSize+1`: the extra sentinel row is how the caller detects that a
next page exists without a second round trip — **drop it** before returning rows to your own
caller.

### Page tokens: `OrderValuesFromRow` and building the next-page token

```go
func OrderValuesFromRow(row *spanner.Row, order []ordering.ColumnOrder) ([]any, error)
func EncodePageToken(t PageToken, fingerprint uint64) string
func DecodePageToken(s string, fingerprint uint64) (PageToken, error)
```

```go
type PageToken struct {
	OrderValues []any // last row's ORDER BY column values, in clause order
	KeyValues   []any // last row's key values (tiebreaker)
}
```

> **This is the one place a hand-written table body is easy to get subtly wrong on a
> multi-column key.** The next-page token's `OrderValues` **must** come from
> `OrderValuesFromRow(lastRow, order)`, where `order` is the *full effective order* returned by
> `BuildList` — not a hand-picked subset, and not just the columns the caller's `OrderBy`
> named. `PageToken.KeyValues` comes from the last row's `Key.KeyValues()`. Building
> `OrderValues` any other way (e.g. only the caller's explicit `OrderBy` columns, omitting the
> appended key tiebreakers) silently corrupts pagination the moment two rows tie on the
> caller's order — the cursor predicate assumes `OrderValues` lines up 1:1, in order, with
> `order`.

### Shape of a consumer table

A consumer table composes the pieces above; this sketch shows the two methods that most often
get pagination wrong — `Read` and `List` — for a table keyed by a single string column and
storing a proto resource, no policy column configured. Imports omitted for brevity.

```go
type BookTable struct {
	client  *spanner.Client
	spec    spanneradapter.KeySpec
	scanner spanneradapter.Scanner[*bookpb.Book]
	stmts   spanneradapter.StatementBuilder
}

func NewBookTable(client *spanner.Client, parser *filtering.Parser) *BookTable {
	spec := spanneradapter.StringKeySpec("key")
	return &BookTable{
		client: client,
		spec:   spec,
		scanner: spanneradapter.Scanner[*bookpb.Book]{
			Spec: spec, Codec: spanneradapter.ProtoCodec[*bookpb.Book](),
			ResourceColumn: "Book", PolicyColumn: "", // no policy column on this table
		},
		stmts: spanneradapter.StatementBuilder{
			TableName: "Books", Spec: spec, ResourceColumn: "Book", PolicyColumn: "", Parser: parser,
		},
	}
}

func (t *BookTable) Read(ctx context.Context, key protodb.Key) (*protodb.Row[*bookpb.Book], error) {
	row, err := spanneradapter.ReadRowByKey(ctx, t.client, "Books", spanneradapter.ToKey(key), t.scanner.Columns())
	if err != nil {
		return nil, err
	}
	return t.scanner.ScanRow(row)
}

func (t *BookTable) List(ctx context.Context, opts protodb.ListOptions) ([]*protodb.Row[*bookpb.Book], string, error) {
	stmt, order, fingerprint, err := t.stmts.BuildList(opts)
	if err != nil {
		return nil, "", err
	}
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = protodb.DefaultPageSize
	}

	it := spanneradapter.Query(ctx, t.client, stmt)
	defer it.Stop()

	// raws is kept parallel to rows so OrderValuesFromRow can be called on
	// the raw *spanner.Row once the LIMIT+1 sentinel is dropped below — a
	// protodb.Row alone doesn't carry non-key order-column values.
	var rows []*protodb.Row[*bookpb.Book]
	var raws []*spanner.Row
	for {
		raw, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, "", spanneradapter.ErrorToStatus(err)
		}
		row, err := t.scanner.ScanRow(raw)
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, row)
		raws = append(raws, raw)
	}

	var nextPageToken string
	if int32(len(rows)) > pageSize {
		// The sentinel row proves a next page exists; drop it before
		// returning to the caller.
		rows, raws = rows[:pageSize], raws[:pageSize]

		// CRITICAL: OrderValues must come from OrderValuesFromRow over the
		// FULL effective order BuildList returned — never a hand-picked
		// subset of it — or pagination silently corrupts on a multi-column
		// key/order the moment two rows tie on the caller's OrderBy.
		// KeyValues come from the last kept row's own key.
		orderValues, err := spanneradapter.OrderValuesFromRow(raws[len(raws)-1], order)
		if err != nil {
			return nil, "", err
		}
		nextPageToken = spanneradapter.EncodePageToken(spanneradapter.PageToken{
			OrderValues: orderValues,
			KeyValues:   rows[len(rows)-1].Key.KeyValues(),
		}, fingerprint)
	}
	return rows, nextPageToken, nil
}
```

## `memadapter` and `protodbtest`: testing

### `memadapter`

An in-memory `protodb.ResourceTable[R]` — a test double, and the second (non-Spanner) adapter
proving the `ResourceTable` seam:

```go
type strKey string
func (k strKey) KeyValues() []any { return []any{string(k)} }

table := memadapter.New[*bookpb.Book](memadapter.Config{
	KeyColumns: []string{"key"}, // must match the order of Key.KeyValues()
})
```

Use it to test service logic against a `protodb.ResourceTable[R]` without a real database:

```go
func TestBookService_Create(t *testing.T) {
	table := memadapter.New[*bookpb.Book](memadapter.Config{KeyColumns: []string{"key"}})
	svc := NewBookService(table)

	_, err := svc.CreateBook(ctx, &pb.CreateBookRequest{ /* ... */ })
	if err != nil {
		t.Fatal(err)
	}
}
```

**Limits, deliberate rather than gaps to fill later:**

- **Filters and non-key ordering are opt-in.** Without `Config.ResourceColumn`, a non-empty
  `Filter` returns `Unimplemented`; without `Config.Columns`, ordering by a non-key column does
  too. See below.
- **One transaction at a time, no retries.** `memadapter.NewTransactionRunner()` holds one
  package-global mutex for `fn`'s entire execution and restores every table's snapshot if `fn`
  returns an error. A nested `RunTransaction` joins the outer one, as with `spanneradapter`. `fn`
  runs exactly once (no abort/retry simulation), which is a valid degenerate case of
  `ReadModifyWrite`'s "may run more than once" contract, but means memadapter cannot exercise
  retry-related bugs.

#### Filtering and ordering in memory

Configure the table the way the Spanner table is configured, and `Filter` and `OrderBy` behave
the same way, with filtering applied before paging:

```go
table := memadapter.New[*bookpb.Book](memadapter.Config{
	KeyColumns:     []string{"key"},
	ResourceColumn: "Book", // filters say Book.title, Book.create_time, ...
	FilterIdentifiers: []filtering.Identifier{ // same identifiers as the Spanner table's parser
		filtering.Timestamp("Book.create_time"),
		filtering.Timestamp("Book.delete_time"),
	},
	Columns: map[string]func(protodb.Key, any) any{ // stand-ins for generated columns
		"create_time": func(_ protodb.Key, r any) any {
			if t := r.(*bookpb.Book).GetCreateTime(); t != nil {
				return t.AsTime()
			}
			return nil // NULL: sorts first ascending, last descending
		},
	},
})

rows, next, err := table.List(ctx, protodb.ListOptions{
	Filter:  "Book.delete_time = NULL",
	OrderBy: "create_time desc",
})
```

A filter path is a key column, `key` (the first key value), `Book` or a field below it (read with
protoreflect, so `R` must be a proto message), or a `Columns` name. An unset message field, or an
unset field with explicit presence, is `NULL`; a plain proto3 scalar reads its default, as in
Spanner. Repeated and map fields, and filter forms the in-memory evaluator cannot reproduce exactly,
return `Unimplemented`. A `Columns` function returns `nil`, a string, bool, number, or `time.Time`;
any other type, or a mix of types across rows, fails with `FailedPrecondition`. Column functions
must not call back into memadapter.

### `protodbtest`

A conformance suite any `ResourceTable[R]` implementation runs against itself — the behavioral
contract, written as ordinary Go test code rather than a checklist:

```go
func TestConformance(t *testing.T) {
	protodbtest.Conformance[*bookpb.Book]{
		NewTable: func(t *testing.T) protodb.ResourceTable[*bookpb.Book] {
			return newBookTable(t) // fresh table per subtest
		},
		Runner: func(t *testing.T) protodb.TransactionRunner {
			return txRunner // nil skips the transactional subtests
		},
		MakeRow: func(i int) *protodb.Row[*bookpb.Book] {
			// keys must be strictly ordered by i under the table's default order
			return &protodb.Row[*bookpb.Book]{Key: strKey(fmt.Sprintf("k%03d", i)), Resource: &bookpb.Book{Id: int64(i)}}
		},
		Equal:          func(a, b *bookpb.Book) bool { return proto.Equal(a, b) },
		SupportsFilter: true, // false asserts Filter returns Unimplemented instead
	}.Run(t)
}
```

The suite is backend-agnostic: it depends only on `protodb` and `testing`, never on a specific
adapter. Run it against `memadapter` unconditionally (fast, no external dependency); run it
against a Spanner-backed table gated behind an environment variable (emulator or a CI-provisioned
database, at the consumer's discretion) since it needs a live backend.

### `protobundle` and `spannertest`: testing your tables on the emulator

Two packages turn the Spanner-emulator setup every consumer used to hand-write into one call.

- **`protobundle`** builds the proto bundle a database needs before a table can declare PROTO
  or ENUM columns. `protobundle.New(roots...)` follows every message- and enum-typed field from
  the root messages, nested enums and the messages containing them included and map entries
  skipped, and gives you the `CREATE PROTO BUNDLE` statement (`CreateStatement`) and the
  `FileDescriptorSet` bytes (`Descriptors`). For a top-level enum that only an ENUM column uses,
  pass its descriptor to `protobundle.NewFromDescriptors(librarypb.Genre(0).Descriptor())`. It reads the registry linked into your binary, so no `.proto` files or
  `protoc` are needed, and it has no test or Docker dependencies, so migration code can use it
  too.
- **`spannertest`** finds or starts an emulator and gives each test a fresh database.
  `spannertest.NewDatabase(t, bundle, ddl...)` returns a `*spanner.Client` and drops the
  database when the test ends.

```go
func TestShelves(t *testing.T) {
	bundle, err := protobundle.New(&librarypb.Shelf{})
	if err != nil {
		t.Fatal(err)
	}
	client := spannertest.NewDatabase(t, bundle,
		"CREATE TABLE Shelves (`key` STRING(MAX) NOT NULL, Shelf `alis.library.v1.Shelf`, "+
			"create_time TIMESTAMP AS (TIMESTAMP_SECONDS(Shelf.create_time.seconds)) STORED) PRIMARY KEY (`key`)")
	runShelfTests(t, client) // your tests against the table
}
```

`librarypb.Shelf` stands in for your own resource message.

The emulator tests are opt-in, so a plain `go test` needs no Docker:

| Variable | Behaviour |
|---|---|
| `SPANNER_EMULATOR_HOST` set | Use that emulator; no container is started. |
| else `SPANNERTEST_EMULATOR` non-empty | Start one emulator for the whole test binary with [testcontainers-go](https://golang.testcontainers.org) (Docker required). A container that cannot start fails the test. |
| `SPANNERTEST_EMULATOR_IMAGE` | Override the pinned image, `spannertest.DefaultImage`. |
| neither | `t.Skip`. |

The testcontainers reaper removes the container when the test binary exits. With
`TESTCONTAINERS_RYUK_DISABLED=true` it stays running and has to be removed by hand.

**Code that builds its own Spanner client.** `spannertest` never sets `SPANNER_EMULATOR_HOST`:
the client it returns connects through explicit options, so tests may call `t.Parallel()`. Most
services create their own client from the environment instead. For those, set the variable in
the test (which then must not be parallel) and hand over the database name:

```go
client := spannertest.NewDatabase(t, bundle, ddl...)
t.Setenv("SPANNER_EMULATOR_HOST", spannertest.Host(t))
svc := newService(ctx, client.DatabaseName())
```

**The emulator cannot read 32-bit integer proto fields** (int32, uint32, sint32, fixed32,
sfixed32), such as `google.protobuf.Timestamp.nanos`. A STORED generated column that reads one
makes every write fail with an opaque `Unexpected error in RPC handling`, and a CHECK constraint
reading one may do the same, so `NewDatabase` rejects both before creating anything and names
the table, the column or constraint, and the path. Read `.seconds` instead, as above.

The check parses each statement with
[`spansql`](https://pkg.go.dev/cloud.google.com/go/spanner/spansql) and covers generated columns
and CHECK constraints in `CREATE TABLE` and in `ALTER TABLE ... ADD COLUMN` / `ADD CONSTRAINT`,
for paths that start at one of the table's proto columns (quoted or not, matched
case-insensitively). A statement `spansql` cannot parse, such as one with an `ARRAY<proto>`
column, a schema-qualified table name or a backticked path head, is not checked: `NewDatabase`
logs it and carries on.

Queries that read such a field fail with `Type not found: INT32` or `UINT32`; wrap errors with
`spannertest.Explain(err)` to get a hint. `Explain` also hints on
`Unexpected error in RPC handling`, which the emulator raises for other internal errors too, so
its hint says the error may come from such a read.

### Running the Spanner conformance test

This repository does exactly that for itself: `spanneradapter`'s own test package builds a
reference `ResourceTable[string]` out of the building blocks above (the "shape of a consumer
table" sketch, materialized) and runs the `protodbtest` suite against a real Spanner, plus a
multi-column tied-order pagination check that executes the null-safe keyset cursor for real.
Its databases come from `spannertest`, with a bundle built by
`protobundle.New(&iampb.Policy{}, &databasepb.Backup{})`, so it is skipped unless `SPANNER_EMULATOR_HOST` or
`SPANNERTEST_EMULATOR` is set. `PROTODB_SPANNER_CONFORMANCE`, which this repository used before
`spannertest`, still starts an emulator.

```sh
# Start one emulator per package (each is its own test binary) via Docker:
SPANNERTEST_EMULATOR=1 go test ./spanneradapter/ ./spannertest/ -v

# Or reuse an emulator you are already running:
SPANNER_EMULATOR_HOST=localhost:9010 go test ./spanneradapter/ ./spannertest/ -v
```

## Migrating from v2.0.x

### Breaking-surface summary

| Old (v2.0.x) | New |
|---|---|
| `BaseResourceTable[R]` / `ResourceTable[R proto.Message]` split | single `ResourceTable[R any]` |
| `BaseResourceRow[R]` / `ResourceRow[R proto.Message]` | `Row[R]` — pure data, no methods |
| `row.Update(ctx)` / `row.Delete(ctx)` | `table.Write(ctx, row)` / `table.Delete(ctx, row.Key)` |
| `RowKey`, `RowKeyFactory`, `SpannerRowKeyFactory` | `Key`, `spanneradapter.KeySpec` (`StringKeySpec`, `KeySpecFor`) |
| `Query(ctx, ...)` | `List(ctx, ListOptions{...})` |
| `Stream` → channel-backed `StreamResponse[T]` (`Next()`/`io.EOF`) | `Stream` → `iter.Seq2[*Row[R], error]` (`for row, err := range ...`), traverses everything, no hidden cap |
| `Merge` / `ApplyReadMask` as `ResourceRow` methods | `protodb.Merge` / `protodb.ApplyReadMask` free functions |
| `SpannerErrorToStatus(err)` | `spanneradapter.ErrorToStatus(err)` |
| `WritePolicy` / `BatchWritePolicies` | `WritePolicies(ctx, entries ...PolicyEntry)` |

### Stream wire-behavior change

The old `Stream` silently capped at 100 rows behind its channel-backed iterator in some code
paths. **The new `Stream` has no cap — it traverses every matching row.** A migrated or
regenerated `Stream` call over a table with more than 100 matching rows will now return more
rows than it used to. Any caller that depended on the old cap (intentionally or not) needs to
add its own limit via `List` instead.

### Page tokens are invalidated across the upgrade

Page tokens are opaque and fingerprinted differently in the new implementation (keyset-based,
not offset-based). **Old tokens do not decode under the new code — they return
`InvalidArgument`.** Do not persist page tokens across this upgrade (e.g. in a client's saved
UI state or a resumable job): any token minted before the upgrade must be discarded, and callers
should be prepared to handle an `InvalidArgument` from a stale token by restarting from the
first page.

### Policy is per-row, not table-wide

There is no more table-level "has policies" flag. A table either configures a `PolicyColumn` (a
non-empty string) on its `Scanner`/`StatementBuilder`, in which case every row may carry a
policy, or it leaves `PolicyColumn` as `""`, in which case the table has no policy column at
all. `Row.Policy` is `nil` for "no policy on this row" regardless.

### `Merge` / `ApplyReadMask` are free functions now, and `ApplyReadMask` no longer mutates the caller's mask

```go
// Old: row.Merge(updated, paths...); row.ApplyReadMask(mask, ignoredPaths...)
// New:
protodb.Merge(dst, src, paths...)
err := protodb.ApplyReadMask(msg, mask, ignoredPaths...)
```

`ApplyReadMask` clones `mask` before normalizing it — the caller's `*fieldmaskpb.FieldMask` is
never modified, so the same mask value can safely be reused across multiple rows in a list
response. (In the old implementation, reusing a mask across rows could accumulate mutations.)

### `SpannerErrorToStatus` moved

```go
// Old: protodb.SpannerErrorToStatus(err)
// New: spanneradapter.ErrorToStatus(err)
```
