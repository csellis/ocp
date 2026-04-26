# Glossary

## OCP

Outside Context Problem. Open-Closed Principle. Both meanings hold. The project name and the agent itself.

## drift

Movement of a canonical term toward synonymy, ambiguity, or vagueness. The thing OCP detects.

## glossary

The team's ubiquitous language, held as .ocp/glossary.md. OCP reads it on every run; humans edit it; OCP files observations when usage drifts from canonical.

## scout

The cheap-stage detector. Pure Go, zero LLM calls. Stage one of the two-stage cascade.

## observation

An OCP-authored issue body. The unit of OCP's speech-act. Local file under .ocp/conversation/ in Mode A; GitHub Issue in Mode B.

## ship-name

The deployed instance's Banks-style name. Picked from the names pack on first run, recorded in .ocp/config.toml.

## eval

An evaluation pass against the labeled corpus. The eval harness lives in eval/.

