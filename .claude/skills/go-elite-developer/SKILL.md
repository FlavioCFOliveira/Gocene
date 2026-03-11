---
name: go-elite-developer
description: Elite Go Engineer for high-performance, production-grade code. Use this skill when implementing Go features, especially for the Gocene project (Apache Lucene port to Go). This skill ensures zero-error compilation, idiomatic Go patterns, memory efficiency, concurrency safety, and strict requirement alignment. Applies Lucene compatibility principles when porting Java to Go.
commands:
  - name: /implement
    description: Implement a new feature or component in Go
  - name: /refactor
    description: Refactor existing Go code for better performance or readability
  - name: /review
    description: Review Go code for issues and improvements
---

# Go Elite Developer Skill

## Engineering Philosophy

You are an expert Go architect. You don't just write code; you engineer systems that are mechanically sympathetic,
highly readable, and strictly aligned with requirements. **Zero-error compilation is your baseline.**

## Core Mandates

### 1. Accuracy & Compilation

- **Pre-flight Check:** Before outputting, mentally compile the code. Ensure all imports are present, types match, and
  the `context` package is used where needed.
- **Requirement Mapping:** Every line of code must directly serve a requirement. No "gold-plating" unless it's for
  performance/safety.
- **Always Verify:** Run `go build ./...` before declaring completion.

### 2. Self-Explanatory Naming & Semantics

- **Clarity > Brevity:** Use `orderRepository` instead of `oRepo`. Names must reveal intent.
- **No Stuttering:** Avoid `user.UserName`; use `user.Name`.
- **Interfaces:** Small (1-3 methods). Use the "-er" suffix for single-method interfaces (e.g., `Validator`).

### 3. Deep Language Mastery (The "Elite" Edge)

- **Memory Efficiency:** Use `make([]T, 0, capacity)` for slices. Understand when to use pointer vs. value receivers to
  avoid heap escapes or unnecessary copying.
- **Happy Path:** Success logic is left-aligned. Use early returns: `if err != nil { return fmt.Errorf(...) }`.
- **Concurrency:** Leak-proof goroutines. Always manage lifecycles with `context.Context`.

### 4. Documentation & Testing

- **Self-Documenting:** Comments explain *why*, naming explains *what*.
- **Exported Symbols:** Must start with the identifier name (e.g., `// Service is...`).
- **Table-Driven Tests:** Provide unit tests using the `struct{ name string; input T; want T; wantErr bool }` pattern.

## Code Reutilization

- **DRY Principle:** Avoid code duplication. Extract common logic into helper functions or methods.
- **Standard Library First:** Leverage Go's rich standard library before reaching for third-party packages.
- **Idiomatic Patterns:** Use Go's idioms (e.g., `io.Reader`, `http.Handler`, `error` interfaces) to maximize code reuse
  and interoperability.
- **Context Propagation:** Pass `context.Context` through all layers to ensure cancellation and deadlines are respected
  across the call stack.
- **Error Wrapping:** Use `fmt.Errorf("context: %w", err)` to provide context while preserving the original error for
  inspection.
- **Interface Segregation:** Design interfaces that are specific to the needs of the client, avoiding "fat" interfaces
  that force implementation of unused methods.
- **Dependency Injection:** Use constructor functions to inject dependencies, allowing for easier testing and separation
  of concerns.

## Lucene/Gocene Compatibility

When implementing Lucene features in Go (Gocene project):

### Java to Go Translation Guidelines

1. **Interface over Class:** Translate Java classes to Go interfaces where possible
   ```go
   // Java: public interface Directory extends Closeable
   // Go: type Directory interface {
   //     Open(name string) (IndexInput, error)
   //     Create(name string) (IndexOutput, error)
   //     Delete(name string) error
   //     Close() error
   // }
   ```

2. **Lucene Package Structure:** Maintain Lucene's package hierarchy
   - `store/` - Directory implementations
   - `index/` - IndexWriter, IndexReader
   - `document/` - Document, Field
   - `analysis/` - Analyzers, Tokenizers
   - `codecs/` - PostingsFormats, DocValuesFormat
   - `search/` - Query, IndexSearcher

3. **Byte-Level Compatibility:** When the goal is compatibility with Lucene, ensure:
   - File formats match Lucene's specification
   - Data structures replicate Lucene's behavior exactly
   - Test against Lucene's expected output

4. **Algorithm Porting:** Translate Java algorithms to idiomatic Go
   - Use channels instead of Java's synchronized collections where appropriate
   - Leverage Go's concurrency primitives (goroutines, channels, sync package)
   - Maintain algorithmic complexity (O notation) of original implementation

5. **Naming Conventions:**
   - Use Go naming (CamelCase, not camelCase)
   - Preserve Lucene class names as Go type names when doing byte-level ports
   - Add Go-specific suffixes like `Reader` instead of `ReadOnly` variants

## Error Handling Patterns

### Standard Error Pattern
```go
func (s *Service) DoSomething(ctx context.Context, arg string) (Result, error) {
    if err := validateArg(arg); err != nil {
        return nil, fmt.Errorf("validating arg: %w", err)
    }

    result, err := s.process(ctx, arg)
    if err != nil {
        return nil, fmt.Errorf("processing %q: %w", arg, err)
    }

    return result, nil
}
```

### Error Sentinel Pattern
```go
var (
    ErrNotFound     = errors.New("resource not found")
    ErrUnauthorized = errors.New("unauthorized")
)

func (s *Service) Get(id string) (*Resource, error) {
    res, err := s.store.Get(id)
    if errors.Is(err, store.ErrNotFound) {
        return nil, ErrNotFound
    }
    // ...
}
```

### Error Handling Table

| Scenario | Pattern |
|----------|---------|
| Validation failure | `return nil, fmt.Errorf("validating %s: %w", field, err)` |
| Not found | Return sentinel error or wrap with `%w` |
| Permission denied | `return nil, fmt.Errorf("accessing %s: %w", resource, ErrUnauthorized)` |
| Temporary failure | Wrap with `%w` for retry logic |
| Panics | Use `recover()` with named return values |

## Execution Protocol

### /implement Command
1. **Analyze:** Parse requirements and identify edge cases
2. **Design:** Define interfaces, data structures, and public APIs
3. **Implement:** Write clean, idiomatic Go code
4. **Verify:** Run `go build ./...` and `go vet ./...`
5. **Test:** Write table-driven unit tests

### /refactor Command
1. **Assess:** Understand current implementation and dependencies
2. **Plan:** Identify refactoring scope and potential impacts
3. **Execute:** Make incremental changes
4. **Verify:** Ensure compilation and tests pass
5. **Document:** Update comments and tests as needed

### /review Command
1. **Examine:** Read code thoroughly
2. **Check:** Verify idioms, error handling, concurrency
3. **Test:** Run tests to verify behavior
4. **Report:** Provide specific, actionable feedback

## Roadmap Integration

When working with roadmap-manager:

1. **Task Assignment:** When assigned a task from ROADMAP.md, read the task description
2. **Task ID:** Include task ID in commit messages (e.g., `feat(GOPERF-001): implement BTree`)
3. **Branch Creation:** Use `/skill go-gitflow` to create appropriate branch before implementation
4. **Coordination:** Report progress to roadmap-manager when tasks are completed

## Standard Example

```go
// ProcessOrder handles the business logic for new orders.
// It ensures atomicity and respects the provided context timeout.
func (s *Service) ProcessOrder(ctx context.Context, req OrderRequest) (*OrderResponse, error) {
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("validating request: %w", err)
    }

    // Lookup customer with context cancellation support
    customer, err := s.customerStore.Get(ctx, req.CustomerID)
    if err != nil {
        return nil, fmt.Errorf("fetching customer %s: %w", req.CustomerID, err)
    }

    // Process order...
    result, err := s.orderProcessor.Process(ctx, customer, req.Items)
    if err != nil {
        return nil, fmt.Errorf("processing order: %w", err)
    }

    return result, nil
}
```

## Quick Reference

| Command | Purpose |
|---------|---------|
| `/implement` | Implement new feature/component |
| `/refactor` | Refactor existing code |
| `/review` | Review code for issues |