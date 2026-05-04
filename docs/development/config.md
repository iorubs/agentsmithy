# Config Design

Developer guide for the `.agentsmithy.yaml` config format: schema
design, versioning, and how to extend it. For the user-facing field
reference, see the auto-generated docs in
[`docs/user/reference/config/`](../user/reference/config/README.md).

## Single Source of Truth: the `agentsmithy` Struct Tag

Every yaml-tagged config field carries an `agentsmithy` struct tag
that declares validation rules and defaults. Fields without
`required` are implicitly optional:

```go
// Hard cap on iterations for kind: loop, or guard-retry budget for kind: autonomous.
MaxIterations int `yaml:"maxIterations,omitempty" agentsmithy:"min=1"`
```

Tag format:

| Tag value                              | Meaning                                            |
|----------------------------------------|----------------------------------------------------|
| `agentsmithy:"required"`               | Field must be present                              |
| `agentsmithy:"default=VALUE"`          | Omitted → defaulted to VALUE                       |
| `agentsmithy:"oneof=GROUP"`            | Exactly one field per GROUP must be set            |
| `agentsmithy:"oneof?=GROUP"`           | At most one field per GROUP may be set (zero OK)   |
| `agentsmithy:"min=N"`                  | Numeric value must be >= N                         |
| `agentsmithy:"ref=path1\|path2"`       | Value must reference a key under one of the paths  |
| `agentsmithy:"notreserved"`            | Map key must not collide with a reserved name      |
| `agentsmithy:"typed-as=TYPE"`          | Doc-only override of the rendered type             |

Two consumers read this tag:

| Consumer | What it does |
|----------|--------------|
| **schema.Process** | Single call that applies defaults, validates required fields, enum values, oneof groups, min bounds, and reference constraints; recurses into nested structs, maps, and slices automatically |
| **Doc generator** | Parses tags via go/ast to build the user-facing reference tables |

This eliminates drift between documentation, validation, and
defaulting: change the tag once, all consumers update
automatically.

### Enum types and the `Valuer` interface

Named string types with a fixed set of valid values implement the
`schema.Valuer` interface:

```go
func (Provider) Values() []string {
    return []string{string(ProviderOllama), string(ProviderOpenAI)}
}
```

`schema.Process` checks non-zero fields whose type implements
`Valuer`; no per-field validation code is needed. Adding a new
enum value means updating the const block and the `Values()`
method in one place.

### Mutual exclusivity and the `oneof` tag

Fields that form a mutually-exclusive group are tagged `oneof=GROUP`,
where GROUP is an arbitrary label tying them together. Fields
sharing the same group name on the same struct are checked: exactly
one must be non-zero.

```go
Autonomous *Autonomous `yaml:"autonomous,omitempty" agentsmithy:"oneof=kind"`
Sequential *Sequential `yaml:"sequential,omitempty" agentsmithy:"oneof=kind"`
Parallel   *Parallel   `yaml:"parallel,omitempty"   agentsmithy:"oneof=kind"`
Loop       *Loop       `yaml:"loop,omitempty"       agentsmithy:"oneof=kind"`
Orchestrator *Orchestrator `yaml:"orchestrator,omitempty" agentsmithy:"oneof=kind"`
```

The `?` suffix (`oneof?=GROUP`) makes the group optional. The doc
generator renders these fields as **oneof** in the Required column.

## Versioning

Each config schema version lives in its own sub-package under
`internal/config/`, with its own types and parser. Each versioned
parser uses strict field validation: unknown keys are errors.
Forward compatibility is handled by version routing: each versioned
`Parse()` method returns the latest `*Config` type, converting if
needed.

### Parse → Default → Validate

1. The caller reads the YAML file and passes raw bytes to
   `config.Parse()`.
2. `config.Parse()` unmarshals just the `version` field, then
   dispatches to the correct versioned parser (e.g. `v1.Parse()`).
3. The versioned parser YAML-decodes with strict mode, calls
   `schema.Process()`, and returns `*Config` already in the latest
   type (for v1 this is direct; v1 **is** the latest).
4. The rest of the codebase operates on the latest types via type
   aliases in `config.go`.

## Design Decisions

### Type aliases

`config.Config` is an alias for the latest version's `Config`. All
downstream consumers import `"internal/config"` without change —
adding a new version only touches the config packages.

### Self-contained versions

Types, parser, validation, and helpers all live together in the
versioned package. The shared `schema` package is version-agnostic;
each `vN` calls it on its own types.

### Strict per-version parsing

Each versioned parser rejects unknown keys as errors
(`yaml.WithKnownFields()`). Forward compatibility comes from
version routing, not lenient parsing.

### Config version ≠ runtime version

The config schema version (`version: "1"`) is independent of the
agentsmithy binary version, the MCP protocol version, and the
provider SDK versions.

### Templated fields

`run`, `output`, and `until` are typed as `TemplateString`. They
parse at validate time against the helper registry (`tool`,
`agent`, `skill`, `prompt`, `coalesce`, `dict`, `list`, plus the
standard Go `text/template` boolean ops). Syntax and helper-arity
errors fail config load, not first-call.

## Adding a New Config Field

1. Add the field to the appropriate struct in
   `internal/config/v1/types.go` with an above-field comment and an
   `agentsmithy:"..."` tag.
2. If the field has a non-zero default, set the tag to
   `agentsmithy:"default=VALUE"`.
3. If the field is required, set `agentsmithy:"required"`.
4. If the value is one of a closed set, define a named string type
   that implements `Values() []string`.
5. Add any cross-field semantic validation as a `Validate() error`
   method on the enclosing struct.
6. Run `go run ./cmd/gen-docs`. The user reference updates
   automatically.

## Adding a New Config Version

When the config schema needs breaking changes:

1. Create `internal/config/vN/` with its own `types.go`, `parse.go`,
   and `const Version = "N"`.
2. Add `agentsmithy` tags to all yaml-tagged fields.
3. Implement `Parse()`: YAML-decode with strict mode, call
   `schema.Process`, convert the result to the **latest** `*Config`
   type, and return it.
4. Update the previous version's `parse.go` to convert into vN's
   types before returning.
5. Update the type aliases in `config.go` to point to vN.
6. Register `vN.Schema{}` in `config.Parse()`.
7. Run `go run ./cmd/gen-docs`. User-facing reference updates
   automatically.
