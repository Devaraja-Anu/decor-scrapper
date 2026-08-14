# Agents.md


Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification


## 5. Guardrails

- Never invent an API, function, config option, or library behavior. If you're not
  certain it exists, check the docs/source — don't assume from pattern-matching.
- Before writing new logic, check if something equivalent already exists in the
  codebase. Reuse or extend it rather than duplicating.
- Don't add a new dependency to solve something solvable in a few lines. If a
  dependency is genuinely the right call, say so and why before adding it.
- "Verified" means actually run it — tests, linter, build — not just re-reading the
  diff and asserting it looks right.


## 6. Documentation


- Documentation if any should be in the docs file
- Update docs with markdown files for each new major feature completed

  
**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.


## 7. Project goals

This is a scrapper project. this will be used to scrap the data from the websites in decisions.md. The resulting json file will be used to create an LLM powered website.
The website itself will be entirely nextjs to be deployed on vercel. The site allows users to upload a picture of their room and the LLM will use the information we have scrapped to generate a decorated room image and give us the price tag and ways to purchase it, streamlining the experience. THis is to be a portfolio project.

---
