# Problems Encountered — Deep Dive

This document captures issues encountered during development and their solutions.
Understanding these problems is crucial for working with Temporal effectively.

---

## Problem 1: Potential Deadlock Detected

- Issued a cancellation on UI  
- Trigger a workflow with same command go run cmd/starter/main.go -cmd=start -service=escalation-test


### Error Message

```
Workflow Worker Unhandled Failure
[TMPRL1101] Potential deadlock detected: workflow goroutine "root" didn't yield for over a second
```

### What This Error Means

Temporal workflows run in a **deterministic, single-threaded execution model**. The Temporal SDK 
monitors workflow code and expects it to regularly "yield" — meaning it should call Temporal SDK 
functions (like `workflow.Sleep()`, `selector.Select()`, `future.Get()`) that give control back 
to the Temporal runtime.

If your workflow code runs for more than 1 second without yielding, Temporal assumes something 
is wrong (infinite loop, blocking call, etc.) and fails the workflow task.

### Root Cause in Our Code

The original code had this pattern:

```go
// PROBLEMATIC CODE - Before the loop
timerCtx, cancelTimer := workflow.WithCancel(ctx)
timerFuture := workflow.NewTimer(timerCtx, EscalationTimeout)

for state.Status != models.StatusResolved {
    selector := workflow.NewSelector(ctx)
    
    // ... signal handlers ...
    
    // Handle escalation timer
    if state.Status == models.StatusOpen {
        selector.AddFuture(timerFuture, func(f workflow.Future) {
            // ... escalation logic ...
            
            // ⚠️ PROBLEM: Reassigning variables inside callback
            timerCtx, cancelTimer = workflow.WithCancel(ctx)
            timerFuture = workflow.NewTimer(timerCtx, EscalationTimeout)
        })
    }
    
    selector.Select(ctx)
}
```

### Why This Caused Issues

#### Issue 1: Variable Reassignment During Select()

When `selector.Select(ctx)` is called, it blocks until one of the registered handlers fires.
When the timer fires, the callback executes. Inside that callback, we were reassigning 
`timerFuture` and `cancelTimer`.

The problem: **Temporal replays workflow history to rebuild state**. During replay, Temporal
re-executes the workflow code and expects it to make the same decisions. When variables are
reassigned inside callbacks, the replay can get confused about which timer is which.

```
Timeline:
1. Select() called
2. Timer fires → callback executes
3. Inside callback: timerFuture = NewTimer(...)  ← Reassignment during Select()
4. Select() returns
5. Next loop iteration: uses the new timerFuture

During Replay:
1. Temporal replays events
2. Gets confused: "Which timer is this? The original or the new one?"
3. Non-determinism detected → Deadlock error
```

#### Issue 2: Closure Variable Capture

Go closures capture variables by reference. When we wrote:

```go
selector.AddFuture(timerFuture, func(f workflow.Future) {
    timerFuture = workflow.NewTimer(...)  // Modifying captured variable
})
```

The `timerFuture` inside the callback refers to the SAME variable as outside. Modifying it
during execution can cause the selector to be in an inconsistent state.

#### Issue 3: Cancellation Context Complexity

We were using `workflow.WithCancel()` to create a cancellable timer:

```go
timerCtx, cancelTimer := workflow.WithCancel(ctx)
timerFuture := workflow.NewTimer(timerCtx, EscalationTimeout)

// Later, in ack handler:
cancelTimer()  // Cancel the timer
```

While this pattern is valid, combining it with variable reassignment inside callbacks 
created a complex state machine that was hard for Temporal to track during replay.

### The Fix

We restructured to use a **flag-based pattern** that creates timers at the START of each 
loop iteration, not inside callbacks:

```go
// FIXED CODE
needNewTimer := true
var timerFuture workflow.Future

for state.Status != models.StatusResolved {
    // Create timer at START of loop iteration (deterministic point)
    if state.Status == models.StatusOpen && needNewTimer {
        timerFuture = workflow.NewTimer(ctx, EscalationTimeout)
        needNewTimer = false
    }
    
    selector := workflow.NewSelector(ctx)
    
    // ... signal handlers ...
    
    if state.Status == models.StatusOpen && timerFuture != nil {
        selector.AddFuture(timerFuture, func(f workflow.Future) {
            // Check state hasn't changed
            if state.Status != models.StatusOpen {
                return
            }
            
            // ... escalation logic ...
            
            // ✅ Only set a flag, don't create timer here
            needNewTimer = true
        })
    }
    
    selector.Select(ctx)
}
```

### Why The Fix Works

| Aspect | Before (Problematic) | After (Fixed) |
|--------|---------------------|---------------|
| Timer creation | Inside callback during Select() | At start of loop iteration |
| Variable reassignment | Inside callback | Never inside callbacks |
| Cancellation | Explicit cancel function | Check state before processing |
| Determinism | Non-deterministic during replay | Deterministic |

The key insight: **Create timers at deterministic points in your workflow code** (like the 
start of a loop), not inside callbacks. Use flags to signal when a new timer is needed.

---