# Day 04 — Arrays, Slices & Maps

**Sequence:** Day 01 foundations → Day 02 functions/errors → Day 03 control flow → **Day 04 collections**. Next: Day 05 structs, methods and pointers.

## Outcomes
Distinguish arrays and slices; explain len/cap/backing arrays; use make, append, copy, slicing; avoid aliasing; CRUD maps; comma-ok; aggregate backend data.

## Run
From \`go-backend-daily/\`:
\`\`\`bash
go run ./day-04/collections
go run ./day-04/exercises
\`\`\`

## 1. Analogy & why it matters
- **Array:** shelf with exactly N slots; its size is part of its type.
- **Slice:** flexible window onto a shelf/backing array. It tracks a start pointer, visible length (\`len\`) and extendable capacity (\`cap\`).
- **Map:** labeled lookup table, e.g. SKU → stock or userID → User.
Backend APIs return lists, imports build slices, caches/indexes use maps. Incorrect slice sharing can silently mutate another part of a program.

## 2. Arrays
\`\`\`go
var ids [3]int                 // [0 0 0]
a := [3]int{10, 20, 30}
b := [...]int{10, 20, 30}      // compiler counts
\`\`\`
\`[3]int\` and \`[4]int\` are different types. Arrays are values: assignment copies their elements. Most app code prefers slices.

## 3. Slices
\`\`\`go
s := []int{10, 20, 30}
s[0] = 99
s = append(s, 40)
\`\`\`
A slice is a descriptor (backing-array pointer, length, capacity), not the array itself. \`len\` is visible elements; \`cap\` is how far the slice can extend from its start before reaching the backing array's end.

### make
\`\`\`go
x := make([]int, 3)       // len=3 cap=3; zeroed
y := make([]int, 2, 8)    // len=2 cap=8
var z []int               // nil: len=0 cap=0
z = append(z, 7)           // valid
\`\`\`
For slices, make allocates backing storage and establishes usable length. Nil slices can be appended to.

### append
\`\`\`go
s := []int{1, 2}
s = append(s, 3, 4)
\`\`\`
Always assign append's result: it may reuse the current backing array if capacity allows, or allocate a new one if not. Capacity growth is an implementation detail—never depend on exact growth factors.

## 4. Aliasing (important!)
\`\`\`go
original := []int{10, 20, 30, 40}
view := original[1:3] // [20 30]
view[0] = 999
// original becomes [10 999 30 40]
\`\`\`
Both slices share storage. Reslicing or append within capacity can write into the same array. If append exceeds capacity, Go allocates a new one and the new slice no longer shares that storage. Don't assume append always allocates.

### copy for independent storage
\`\`\`go
source := []int{1, 2, 3}
destination := make([]int, len(source))
n := copy(destination, source)
\`\`\`
copy transfers up to the smaller length; it does not allocate destination. Use make first. This is the usual way to create independent slice storage.

### Remove while preserving order
\`\`\`go
copy(values[i:], values[i+1:])
values = values[:len(values)-1]
\`\`\`
Validate index first. This shifts items and costs O(n). If order doesn't matter, swap-with-last can be O(1), but changes order.

## 5. Maps
\`\`\`go
stock := map[string]int{"SKU-A": 25}
stock["SKU-B"] = 10
stock["SKU-A"] += 2
delete(stock, "SKU-B")
\`\`\`
Map maps comparable keys to values. Keys can be strings, numbers, arrays or structs whose fields are comparable; slices, maps and functions cannot be keys.

### nil map
\`\`\`go
var m map[string]int // reads okay, writes panic
m = make(map[string]int)
m["A"] = 1
\`\`\`
A nil map lookup returns the value type's zero value. A nil map assignment panics. \`make(map[K]V, hint)\` can give an approximate capacity hint (not a limit).

### comma-ok: distinguish missing from zero
\`\`\`go
qty, exists := stock["SKU-Z"]
\`\`\`
A missing key returns zero value; \`exists\` distinguishes missing from a stored zero. Crucial if 0 is valid data.

### map iteration
\`\`\`go
for key, value := range stock { ... }
\`\`\`
Order is not guaranteed. For deterministic logs/reports/tests, collect keys and sort them.

### concurrency caution
Maps are not safe for concurrent writes or read/write without synchronization. Use a mutex or another concurrency-safe design; we'll cover synchronization later.

## 6. Runnable backend example
\`collections/main.go\`:
- fixed array stores weekly load counts;
- slice appends shipment IDs;
- sub-slice demonstrates shared storage;
- copy demonstrates independent destination;
- map adds, updates and deletes stock;
- comma-ok checks presence;
- InventoryItem records represent products;
- aggregateInventory combines repeated SKU quantities and computes total value;
- sorted keys make output stable.

## 7. Exercises (solutions in exercises/solutions.go)
Run: \`go run ./day-04/exercises\`.
1. \`average([]int) (float64,bool)\`: mean; empty returns false.
2. \`removeAt([]string,index)\`: remove safely, preserving order; invalid index returns false.
3. \`frequency([]string) map[string]int\`: trim/lowercase; ignore blanks; count words.
4. \`mergeStock(first,second)\`: add matching SKU quantities into a NEW map; do not mutate inputs.
5. Bonus: explain why a sub-slice can mutate its original; fix it with copy.

## 8. Interview questions
1. Array vs slice? Array fixed-size value; slice descriptor over backing array.
2. Is slice a reference type? More precisely, descriptor includes pointer/len/cap; copies can share storage.
3. len vs cap? Visible count vs max reslice length from current start.
4. append behavior? Reuses storage if capacity sufficient, else allocates; assign returned slice.
5. Does copy allocate? No; destination must already have length.
6. Can nil slice be appended? Yes.
7. Can nil map be written? No; initialize.
8. Missing key vs zero? comma-ok.
9. Map iteration order? No guaranteed order.
10. Are maps concurrency-safe? Not for concurrent writes/read-write without synchronization.
11. Map key restrictions? Comparable types only.
12. Why copy a slice? Avoid accidental shared backing-array mutation.

## 9. Backend use cases
- API response lists: \`[]Shipment\`, \`[]User\`
- batch import: append valid rows to slice
- in-memory cache/index: \`map[userID]User\`
- stock aggregation: \`map[SKU]int\`
- deduplicate IDs: \`map[string]struct{}\`
- group records: \`map[department][]Employee\`
- request validation: comma-ok for lookup and zero-value distinction
- stable report output: sort map keys

## Day 04 checklist
- [ ] Array length is part of type.
- [ ] I can explain len/cap/backing array.
- [ ] I know append may reuse or allocate.
- [ ] I can copy slice elements.
- [ ] I can avoid unintended aliasing.
- [ ] I can initialize/write/read/delete maps.
- [ ] I can use comma-ok and sort keys.
- [ ] I completed exercises 1–4.

**Next:** Day 05 — structs, methods, pointers, receiver semantics and domain models.
