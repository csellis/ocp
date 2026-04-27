# OCP Eval

Labeled corpus and runner for measuring scout's precision and recall on the synonymy task.

Run from the module root:

```
make eval
```

or directly:

```
go run ./eval
```

## Corpus shape

Each subdirectory of `corpus/` is one example:

```
corpus/<NNN-slug>/
├── glossary.md      glossary scout sees
├── testdata/            fixture filesystem to scan
│   └── ...          .go / .md / .toml files
└── expected.json    labeled hits scout should produce
```

`expected.json`:

```json
{
  "hits": [
    {"file": "doc.md", "line": 3, "synonym": "vocabulary", "canonical": "glossary"}
  ]
}
```

`file` is relative to `testdata/`. `line` is 1-indexed. `synonym` is matched case-insensitively. `canonical` matches the glossary entry exactly.

## Adding an example

1. Pick a 3-digit prefix and a kebab-case slug describing the behavior under test.
2. Create the three pieces above.
3. Run `make eval` and confirm the example shows TP/FP/FN you expect.

The corpus should grow when scout grows: a new scout behavior gets a corpus example that proves the new behavior in isolation.

## What the score means

Today's scout is simple (regex word-boundary match), so the corpus is uniformly green. As scout grows (real ambiguity detection, vagueness, codebase-aware scoring) we should expect, and accept, intermediate scores. The eval is regression insurance and a quality target, not a pass/fail gate.
