# NebulaGC Performance Benchmarking

This directory contains performance benchmarking utilities for the NebulaGC server.

## Current Performance Baselines

Based on E2E test runs (48 tests, 260ms total runtime on MacBook Air M2):

### Test Execution Performance
- **Total Tests**: 48 tests (12 database + 36 API-level)
- **Total Runtime**: ~260ms
- **Average per test**: ~5.4ms
- **Database operations**: Fast (< 10ms per operation)
- **API operations**: Fast (< 10ms per operation)

### Observed Characteristics
- SQLite with WAL mode performs well for single-instance workloads
- Foreign key constraints properly enforced with no performance impact
- Transaction commits are fast (< 5ms)
- Complex JOINs (nodes + clusters + tenants) execute quickly
- Test database cleanup is efficient

## Benchmark Infrastructure

The `helpers.go` file provides utilities for:
- Setting up temporary SQLite databases
- Seeding test data (tenants, clusters, nodes, bundles)
- HTTP client for API benchmarking
- Latency statistics calculation (p50, p95, p99)

## Future Benchmarking

For production-grade performance testing, consider:

### 1. Load Testing Tools
Use external tools for realistic load testing:

```bash
# Apache Bench
ab -n 10000 -c 100 http://localhost:8080/api/v1/config/version

# wrk (advanced HTTP benchmarking)
wrk -t4 -c100 -d30s http://localhost:8080/api/v1/config/version

# vegeta (Go-based load testing)
echo "GET http://localhost:8080/api/v1/config/version" | \
  vegeta attack -duration=30s -rate=1000 | \
  vegeta report
```

### 2. Profiling
Use Go's built-in profiling tools:

```bash
# CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./tests/bench/
go tool pprof cpu.prof

# Memory profiling
go test -bench=. -memprofile=mem.prof ./tests/bench/
go tool pprof mem.prof

# Block profiling (mutex contention)
go test -bench=. -blockprofile=block.prof ./tests/bench/
go tool pprof block.prof
```

### 3. Database Performance
For database-specific benchmarking:

```bash
# SQLite performance analysis
sqlite3 nebula.db ".timer ON"
sqlite3 nebula.db "EXPLAIN QUERY PLAN SELECT ..."
sqlite3 nebula.db "PRAGMA optimize"
```

### 4. Continuous Benchmarking
Integrate benchmarks into CI/CD:

```yaml
# GitHub Actions example
- name: Run benchmarks
  run: go test -bench=. -benchmem ./tests/bench/ > bench-results.txt

- name: Compare with baseline
  run: benchstat baseline.txt bench-results.txt
```

## Performance Targets

Based on expected production workloads:

### API Throughput
- **Version checks**: > 1000 req/s
- **Bundle downloads**: > 100 req/s (1MB bundles)
- **Node operations**: > 500 req/s (CRUD)
- **Concurrent connections**: > 1000 simultaneous

### Latency Targets
- **API latency p95**: < 100ms
- **Database query p95**: < 50ms
- **Bundle download**: < 1s for 10MB
- **Transaction commit**: < 10ms

### Resource Usage
- **Memory per cluster**: < 50MB
- **CPU (idle)**: < 5%
- **Database size**: ~100MB per 1000 nodes
- **Lighthouse process**: < 30MB RAM, < 5% CPU

## Optimization Recommendations

### Database Layer
1. **Indexing**: Add indexes on frequently queried columns
   ```sql
   CREATE INDEX idx_nodes_cluster ON nodes(cluster_id);
   CREATE INDEX idx_bundles_version ON config_bundles(cluster_id, version DESC);
   ```

2. **Query optimization**: Use prepared statements for repeated queries

3. **Connection pooling**: Configure appropriate pool size
   ```go
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(5)
   db.SetConnMaxLifetime(5 * time.Minute)
   ```

4. **WAL checkpointing**: Tune checkpoint frequency
   ```sql
   PRAGMA wal_autocheckpoint=1000;
   ```

### API Layer
1. **Response compression**: Enable gzip for large responses
2. **Caching**: Cache static responses (topology for unchanged versions)
3. **Batch operations**: Support bulk node creation
4. **Streaming**: Stream large bundle downloads

### Lighthouse Management
1. **Batch updates**: Don't restart for every small change
2. **File watchers**: Use inotify instead of polling
3. **Process pooling**: Pre-fork pattern for faster restarts

### General
1. **Reduce allocations**: Use sync.Pool for temporary objects
2. **Optimize logging**: Use structured logging with sampling
3. **Profile regularly**: Run CPU/memory profiling in staging

## Running Benchmarks

Currently, the benchmark infrastructure is set up but needs implementation.
To add benchmarks:

1. Create benchmark functions in `*_test.go` files:
   ```go
   func BenchmarkNodeInsert(b *testing.B) {
       db, clusterID := SetupTestDB(b)
       b.ResetTimer()
       for i := 0; i < b.N; i++ {
           // benchmark code
       }
   }
   ```

2. Run benchmarks:
   ```bash
   go test -bench=. -benchmem ./tests/bench/
   ```

3. Compare results:
   ```bash
   go test -bench=. -benchmem ./tests/bench/ > new.txt
   benchstat old.txt new.txt
   ```

## Dependencies

- Go testing framework (`testing.B`)
- SQLite3 driver (`github.com/mattn/go-sqlite3`)
- HTTP client (`net/http`)

## References

- [Go Benchmarking Guide](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [SQLite Performance Tuning](https://www.sqlite.org/optoverview.html)
- [benchstat tool](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
