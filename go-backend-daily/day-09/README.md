# Day 09 — JSON & File I/O (Beginner → Advanced)

**Curriculum:** Day 08 Packages/Modules → **Day 09 JSON & File I/O** → Day 10 HTTP Server & REST APIs.

## Learning outcomes
- Convert Go values to JSON and JSON to Go values.
- Understand struct tags, exported fields, and `omitempty`.
- Separate request DTOs from domain models.
- Decode untrusted input strictly and validate it.
- Read JSON files and replace them safely.
- Explain why JSON files are not a concurrent database.

## 1. Mental model
Imagine JSON as a parcel label from another system and a Go struct as your warehouse's internal record. Decoding checks the label's shape; validation checks whether the values are acceptable. Valid JSON is not automatically valid business input.

## 2. JSON basics
`encoding/json` provides:
- `json.Marshal(value)`: Go value → JSON bytes.
- `json.Unmarshal(data, &destination)`: JSON bytes → Go value (destination pointer required).
- `json.NewDecoder(reader)`: decode from an `io.Reader` stream.
- `json.NewEncoder(writer)`: encode to an `io.Writer`.
- `json.MarshalIndent(value, "", "  ")`: human-readable JSON.

Example:
```go
type Product struct {
    ID   int      `json:"id"`
    Name string   `json:"name"`
    Tags []string `json:"tags,omitempty"`
}
```
Tags define external JSON field names. Only exported fields (uppercase first letter) are encoded/decoded. `omitempty` omits zero/empty values while encoding; it is not required-field validation. `time.Time` uses an RFC 3339 JSON timestamp.

## 3. DTOs and validation
A DTO (Data Transfer Object) is shaped for a boundary such as an API request/response. Keep it separate from a persistence/domain model when responsibilities differ. A create request should not accept server-owned ID or CreatedAt fields.

Decode first, validate second. Missing JSON fields generally remain Go zero values. Validate required strings, numeric ranges, and business rules after decoding.

## 4. Strict JSON request decoding
```go
decoder := json.NewDecoder(r)
decoder.DisallowUnknownFields()
if err := decoder.Decode(&request); err != nil { return err }
if err := decoder.Decode(new(any)); err != io.EOF {
    return errors.New("only one JSON value is allowed")
}
```
- `DisallowUnknownFields` catches misspelled/unexpected keys.
- A second decode must return `io.EOF`; otherwise multiple JSON values were supplied.
- Strictness is an API compatibility choice: strict APIs catch mistakes; tolerant APIs can be more forward-compatible.
- HTTP handlers should cap body size with `http.MaxBytesReader`.
- Never trust client IDs, timestamps, roles, or values that should be server-controlled.

## 5. File I/O
- `os.ReadFile`: simple for small files; loads the complete file into memory.
- `os.WriteFile`: convenient, but direct replacement can expose partial content if interrupted.
- Safer replacement: encode → temporary file in same directory → write → Sync → Close → Rename.
- Check every error; close files; use `filepath.Join` for paths.
- Rename atomicity/durability varies by OS/filesystem. It is not a transaction and does not prevent concurrent lost updates.
- Do not use JSON files as multi-user production storage; use a database and transactions.

## 6. Project files
```text
day-09/jsonio/
├── main.go
├── main_test.go
└── data/products.json
```
Run from the `go-backend-daily` module root:
```bash
go run ./day-09/jsonio
go test ./day-09/jsonio
go vet ./day-09/jsonio
```

## 7. Code walkthrough
- **Product:** domain/output model with JSON tags.
- **CreateProductRequest:** client input DTO; no server-owned ID/time.
- **Validate:** trims/checks SKU, name, positive price.
- **decodeOneJSON:** stream decoder, unknown-field rejection, exactly one JSON value, wrapped errors.
- **readProducts:** ReadFile then Unmarshal; errors include operation context.
- **writeProductsAtomic:** MarshalIndent; ensure directory; CreateTemp; Write/Sync/Close; Rename; cleanup temp file.
- **main:** reads sample catalog, decodes simulated create request, validates, assigns ID/time server-side, persists, prints response JSON.

## 8. Exercises + solutions
1. **Add Description with omitempty.**  
   `Description string `json:"description,omitempty"``; empty string is omitted.
2. **Price maximum 1,000,000.**  
   After positive check: `if req.Price > 1_000_000 { return errors.New("price is too high") }`.
3. **Why does `"price":"10"` fail?**  
   Price is float64; JSON string is not JSON number. Send `"price":10`.
4. **Allow unknown fields.**  
   Remove `DisallowUnknownFields`; more tolerant, but misspellings may silently pass.
5. **Malformed file test.** Write `{not-json` to a temp file and assert `readProducts` returns an error.
6. **Compact response.** Use `json.Marshal(products)` rather than `MarshalIndent`.
7. **Why not JSON as concurrent DB?** Concurrent read-modify-write loses updates; there are no DB transactions, indexes, constraints, or multi-process coordination.

## 9. Interview questions
- **Marshal vs Unmarshal?** Encode Go→JSON bytes vs decode JSON bytes→Go value.
- **Why pointer for Unmarshal?** Decoder must mutate destination.
- **What is omitempty?** Omits zero/empty fields during encoding; does not validate.
- **Decoder vs Unmarshal?** Decoder reads a stream; Unmarshal consumes a byte slice.
- **Reject unknown fields?** `DisallowUnknownFields()`.
- **Reject multiple JSON values?** Decode once, then require second Decode to return io.EOF.
- **Does successful decode mean valid input?** No; run business validation.
- **Why atomic rename?** Reduce risk of partially-written file being observed; not a database transaction.
- **What is DTO?** Boundary-specific data shape, such as API request/response.

## 10. Backend use cases
- REST POST: decode request DTO → validate → service → persist → encode response.
- Startup configuration: read JSON and validate required settings.
- Queue/event consumer: decode message and reject malformed events.
- Export/report generation and JSON test fixtures.
- Small single-process cache/snapshot using atomic replacement.

## 11. Production checklist
- [ ] Request body limit.
- [ ] DTO rather than blindly decoding into DB model.
- [ ] Unknown-field policy decided.
- [ ] Trailing JSON rejected.
- [ ] Validation after decode.
- [ ] Server owns IDs, timestamps, and authorization.
- [ ] Response Content-Type is application/json.
- [ ] Internal errors wrapped; client errors safely mapped.
- [ ] Write/Sync/Close/Rename errors checked.
- [ ] JSON file is not concurrent multi-user storage.

## Next: Day 10 — HTTP Server & REST APIs
Handlers, routes, status codes, request/response lifecycle, graceful shutdown, and an API built on these DTOs.
