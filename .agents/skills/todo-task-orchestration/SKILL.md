---
name: todo-task-orchestration
description: Coordinate implementation tasks from a repository TODO with subagents, isolated feature branches, one-at-a-time user review, and feedback-driven sequencing. Use when managing TODO work across agents or presenting completed task results for review.
---

# TODO Task Orchestration

Use this skill to coordinate implementation work listed in a repository TODO. The coordinator owns task selection, scope, review, and communication with the user; subagents implement scoped code changes.

## Language

- Write this skill and its internal instructions in English.
- Use the language of the current conversation for user-facing progress updates, questions, review pages, and final results.
- Keep source code, identifiers, and original comments in their existing language.

## Workflow

1. Read the repository instructions and TODO list. Preserve existing user changes and identify task dependencies.
2. Delegate code-writing tasks to available subagents using an appropriate lower-cost model. Do not assign the whole TODO queue to one subagent. Split independent tasks across agents when capacity allows; serialize dependent work.
3. Give each task a clear objective, allowed scope, architectural constraints, expected checks, and a separate `feature/<task-name>` branch/worktree based on the current `main`. Do not ask for confirmation to create the required worktree when the user has already established this workflow.
4. Require each subagent to rebase its feature branch onto the latest `main` before submitting its result for coordinator review. Resolve conflicts on that branch and rerun applicable checks after rebasing. Do not accept a stale result as ready for review.
5. The coordinator reviews the diff, checks the reported validation, and creates a `change-review-page` artifact for code changes. Markdown-only tasks do not need a review page. State limitations plainly when checks are blocked.
6. Present only one completed task to the user at a time. Keep other completed tasks queued and do not present them as the next review item yet.
7. Wait for user feedback on the current task before presenting another one. Feedback includes approval, requested changes, questions, or discussion; approval is not required to resume the queue. Apply feedback to the active task, then select the next ready task.
8. Only after explicit user approval may the coordinator promote accepted changes to `main`, rebasing on the latest `main` and fast-forwarding to the approved feature result. Never promote unapproved work.
9. A task is not complete while implementation errors, unresolved review blockers, or failures in required checks remain. Fix them and rerun the affected checks before marking the task complete. If validation is blocked by an external or unrelated repository problem, report the blocker and keep the task incomplete; do not describe it as completed or verified.
10. Update TODO status to reflect actual progress. Do not mark a task complete until the requested behavior is implemented and required review/validation work is finished without errors.

## Handoff format

For each task, report the objective, feature branch, key changes, validation results and blockers, and the review-page path when code changed. Keep the report in the conversation language and make it self-contained.
