# Concurrency Patterns — Go Idiomatic Reference

## When to Use Which Pattern

| Situation | Pattern |
|---|---|
| N jobs, bounded parallelism | Worker Pool |
| Sequential transformations | Pipeline |
| Same job → N workers | Fan-Out |
| N results → 1 stream | Fan-In |
| Limit concurrent resource access | Semaphore |
| N goroutines, first error cancels all | Errgroup |
| Detect blocked goroutines | Heartbeat |
| Cancel when any one of N signals fires | Or-Done |

---

## 1. Worker Pool (Bounded Concurrency)

```go
func WorkerPool(ctx context.Context, jobs <-chan Job, numWorkers int) <-chan Result {
    results := make(chan Result)
    var wg sync.WaitGroup

    for range numWorkers {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case job, ok := <-jobs:
                    if !ok {
                        return
                    }
                    select {
                    case results <- process(job):
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    return results
}
```

**Key:** Close `jobs` to signal workers to stop. Workers never close `results` — the WaitGroup goroutine does.

---

## 2. Pipeline (Chained Stages)

```go
// Each stage: takes a read-only channel, returns a read-only channel
func generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func square(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            select {
            case out <- n * n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Usage
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
for v := range square(ctx, generate(ctx, 1, 2, 3, 4)) {
    fmt.Println(v)
}
```

---

## 3. Fan-Out / Fan-In

```go
// Fan-Out: distribute work from one channel to N workers
func fanOut(ctx context.Context, in <-chan Work, n int) []<-chan Result {
    channels := make([]<-chan Result, n)
    for i := range n {
        channels[i] = worker(ctx, in) // all read from the same `in`
    }
    return channels
}

// Fan-In: merge N channels into one
func merge(ctx context.Context, channels ...<-chan Result) <-chan Result {
    out := make(chan Result)
    var wg sync.WaitGroup

    forward := func(ch <-chan Result) {
        defer wg.Done()
        for r := range ch {
            select {
            case out <- r:
            case <-ctx.Done():
                return
            }
        }
    }

    wg.Add(len(channels))
    for _, ch := range channels {
        go forward(ch)
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

---

## 4. Semaphore (Rate-Limit Concurrency)

```go
// Buffered channel as counting semaphore
sem := make(chan struct{}, maxConcurrent)

for _, item := range items {
    sem <- struct{}{} // acquire
    go func(item Item) {
        defer func() { <-sem }() // release
        process(item)
    }(item)
}

// Drain the semaphore to wait for all goroutines
for range maxConcurrent {
    sem <- struct{}{}
}
```

**Production alternative:** `golang.org/x/sync/semaphore` for weighted semaphores.

---

## 5. Errgroup (Concurrent Errors with Auto-Cancellation)

```go
import "golang.org/x/sync/errgroup"

func fetchAll(ctx context.Context, ids []int64) ([]*User, error) {
    g, ctx := errgroup.WithContext(ctx)
    users := make([]*User, len(ids))

    for i, id := range ids {
        i, id := i, id // shadow for closure
        g.Go(func() error {
            u, err := fetchUser(ctx, id)
            if err != nil {
                return fmt.Errorf("fetchUser %d: %w", id, err)
            }
            users[i] = u
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return users, nil
}
```

**Analogy to JS:** `errgroup.Wait()` is like `Promise.all` — fails on first error and cancels the rest. For "collect all errors" behavior, use a custom aggregator.

---

## 6. Or-Done (Cancel When Any Signal Fires)

```go
// Wraps a channel so you can range over it respecting context cancellation
func orDone(ctx context.Context, ch <-chan interface{}) <-chan interface{} {
    out := make(chan interface{})
    go func() {
        defer close(out)
        for {
            select {
            case <-ctx.Done():
                return
            case v, ok := <-ch:
                if !ok {
                    return
                }
                select {
                case out <- v:
                case <-ctx.Done():
                }
            }
        }
    }()
    return out
}

// Usage: range safely without caring whether ctx or ch finishes first
for v := range orDone(ctx, inputCh) {
    process(v)
}
```

---

## 7. Heartbeat (Detect Blocked Goroutines)

```go
func doWork(ctx context.Context, pulse time.Duration) (<-chan Result, <-chan struct{}) {
    results := make(chan Result)
    heartbeat := make(chan struct{}, 1)

    go func() {
        defer close(results)
        ticker := time.NewTicker(pulse)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                select {
                case heartbeat <- struct{}{}: // non-blocking send
                default:
                }
            case <-ctx.Done():
                return
            default:
                // do actual work
                result := compute()
                select {
                case results <- result:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()

    return results, heartbeat
}
```

---

## 8. Timeout Pattern

```go
// Per-operation timeout using context
func fetchWithTimeout(id int64) (*User, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel() // always call cancel to release resources

    return db.GetUser(ctx, id)
}

// select-based timeout (for channel operations)
select {
case result := <-resultCh:
    return result, nil
case <-time.After(5 * time.Second):
    return nil, ErrTimeout
case <-ctx.Done():
    return nil, ctx.Err()
}
```

**Warning:** `time.After` in a loop creates a goroutine-per-call that leaks until the timer fires. In loops, use `time.NewTimer` and `Reset`:
```go
timer := time.NewTimer(timeout)
defer timer.Stop()
// ... reset with timer.Reset(timeout) between iterations
```

---

## sync.Cond — When Channels Aren't Enough

Use `sync.Cond` when multiple goroutines need to wait for a **condition** to become true (not just for data arrival):

```go
type Queue struct {
    mu    sync.Mutex
    cond  *sync.Cond
    items []Item
}

func NewQueue() *Queue {
    q := &Queue{}
    q.cond = sync.NewCond(&q.mu)
    return q
}

func (q *Queue) Push(item Item) {
    q.mu.Lock()
    q.items = append(q.items, item)
    q.cond.Signal() // wake one waiter
    q.mu.Unlock()
}

func (q *Queue) Pop() Item {
    q.mu.Lock()
    defer q.mu.Unlock()
    for len(q.items) == 0 {
        q.cond.Wait() // releases lock, sleeps, re-acquires on wake
    }
    item := q.items[0]
    q.items = q.items[1:]
    return item
}
```

**Use `Broadcast()`** when the state change matters to all waiters (e.g., shutdown signal). **Use `Signal()`** when only one waiter needs to proceed.
