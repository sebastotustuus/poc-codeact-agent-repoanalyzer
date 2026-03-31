# Decisions, Tradeoffs & Experience

This document explains the thinking behind certain choices in this project — not the technical implementation, but the reasoning, the tradeoffs, and what I learned along the way.

---

## VibeCoding as a development strategy

This project was built using **VibeCoding**: delegating code writing to the LLM while maintaining strategic supervision — defining specs, reviewing design decisions, validating outputs, and iterating through feedback cycles.

The goal was not to skip the engineering work, but to shift it. Instead of writing lines of code, the effort went into writing precise instructions, understanding the domain deeply enough to validate what the LLM produced, and catching misalignments early. The code is the result of several rounds of planning, generation, review, and correction — with the LLM as the implementor and me as the architect.

The takeaway: the constraint with VibeCoding is not execution speed but specification clarity. When you know exactly what you want and why, the LLM can produce high-quality output quickly. When the spec is vague, the output is too.

---

## Leaving CLAUDE.md and the Skill in the repository

Both `.claude/CLAUDE.md` and `.claude/skills/senior-go-idiomatic/` are intentionally left in the repo. They are the actual instructions that guided the LLM during development, and leaving them in makes the methodology transparent rather than hiding it.

The `senior-go-idiomatic` skill is worth noting specifically: it was written by me, not generated. It encodes the Go standards I care about — error wrapping patterns, interface design, concurrency rules, struct memory layout, testing conventions — and it served as the quality bar the LLM had to meet on every file it touched. Sharing it here is part of showing the full picture of how AI-assisted development can work when you invest in the scaffolding.

---

## Going further than a PoC "should"

Knowing this was a proof of concept, certain things could have been left rough. They weren't. The agent has a proper sandbox, a race-detector-clean test suite, idiomatic Go interfaces, a REPL instead of a one-shot CLI, and support for both remote URLs and local paths.

This was intentional — to demonstrate something specific about VibeCoding: the cost of doing things *well* is much closer to the cost of doing them *quickly* than it used to be. The constraint shifts from "how long will this take?" to "do I know clearly enough what I want?" When the specs are solid, quality comes along almost for free. The PoC label became more of a scope limit than an excuse for shortcuts.

---

## Why a repo analyzer

Analyzing a codebase is a task most developers do regularly — understanding unfamiliar code, spotting security risks, auditing dependencies, mapping structure before a refactor. It is also inherently open-ended: you rarely know exactly what you are looking for until you start exploring.

That makes it a strong match for CodeAct. A fixed set of JSON tools would require predicting every useful operation upfront. With bash as the action space, the agent can run `git log`, `grep -r "TODO"`, `find . -name "*.env"`, `wc -l **/*.go`, or any combination — whatever the specific question demands.

The CLI form factor was also deliberate. It revisits a style of tooling that has been somewhat displaced by web UIs, but remains powerful, composable, and scriptable. Security scanning, dependency auditing, architectural reviews — these are tasks where a CLI agent fits naturally into a developer's workflow.

---

## SDK vs raw HTTP

The first implementation used a raw `net/http` client for the Gemini API. Zero external dependencies, full visibility into every byte. This felt like the right call for a PoC with a "minimize deps" rule.

After seeing it work, the switch to `google.golang.org/genai` — the official Go SDK — was straightforward. The tradeoff became clear: the raw client was explicit but duplicated work the SDK already handles correctly (authentication, retries, connection pooling, response parsing). For a project where the goal is to demonstrate the agent pattern — not the HTTP layer — the SDK is the right abstraction. The `internal/gemini` package keeps it behind a one-method interface, so the rest of the code doesn't know or care how the request is made.

---

## Single-shot vs interactive REPL

The initial design ran one analysis, printed a report, and exited. It worked, but it had a ceiling: you got what the model decided to give you, with no way to direct it further.

The shift to an interactive REPL changed the nature of the tool. "Now check for SQL injection." "What does the authentication layer actually do?" "Is this test file actually testing anything?" That follow-up depth is where the real value lives — not in the initial summary, but in the ability to keep pulling threads.

Technically, the change was smaller than expected: lift conversation history from a local variable inside `Analyze()` to a field on the `Agent` struct, add a `Setup()` / `Turn()` split, and replace the one-shot `run()` with a `bufio.Scanner` loop. The CodeAct loop itself didn't change at all. The architecture supported the evolution naturally.

---

## Understanding CodeAct before building it

I was not deeply familiar with CodeAct at the implementation level before this project. I knew it existed and had a rough sense of how it worked, but not the specifics that would matter when building it.

Before writing specs, I spent time with the original paper and its comparisons to ReACT. ReACT is also a reasoning + action framework, but it assumes a fixed, predefined set of tools. The model picks from a menu. CodeAct removes the menu: the model writes arbitrary code, and the environment executes it.

That distinction shaped several decisions:

- The system prompt should not define tools. It should define the *format* (bash blocks) and the *goal*, then step back.
- The observation format should look like a terminal, not a structured API response — because that is what the model was trained on.
- Self-debugging through stderr is not an edge case to handle away — it is one of the core advantages of the paradigm, and the prompt should leave room for it.

Understanding the paradigm before building it meant the specs were coherent from the start. Without that grounding, the same agent could have been built by accident — and then it would have been much harder to evaluate whether it was actually working the way CodeAct is supposed to work.

The broader principle I try to hold: without prior knowledge, there is no scalable product. You can build something that runs without understanding it, but you cannot maintain it, extend it, or catch its failure modes. The investment in understanding paid back in the quality of the prompts, the architecture decisions, and the ability to course-correct when the LLM drifted.

---

## References

- Wang et al., [Executable Code Actions Elicit Better LLM Agents](https://arxiv.org/abs/2402.01030), 2024
- [EmergentMind — CodeAct Agent Framework](https://www.emergentmind.com/topics/codeact-agent-framework)
