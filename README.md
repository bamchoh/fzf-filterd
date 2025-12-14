# fzf-filterd

fzf-filterd is a lightweight background daemon that exposes fzf’s fuzzy filtering algorithm over JSON-RPC, without invoking the fzf UI.

It is designed to be used as a backend for custom GUIs, launchers, and other frontends that need fast, high-quality fuzzy matching while retaining full control over window management and user experience.

---

## Motivation

Calling the `fzf` command directly from GUI applications often leads to practical issues, especially on Windows:

- Console windows briefly appearing
- Focus being stolen or windows being sent to the background
- Limited control over rendering and interaction

fzf-filterd solves these problems by extracting and reusing only fzf’s filtering algorithm, allowing frontends to implement their own UI while keeping the matching behavior consistent with fzf.

---

## Features

- Uses fzf’s native fuzzy matching algorithm (FuzzyMatchV2)
- Runs as a background daemon
- JSON-RPC interface
- IPC via Win32 Named Pipes (Windows)
- No fzf process invocation
- GUI-agnostic design

---

## Architecture

```
+-------------+        JSON-RPC        +---------------+
|   Frontend  | <-------------------> |  fzf-filterd  |
| (GUI / CLI) |    (Named Pipe IPC)   |  (Go daemon)  |
+-------------+                        +---------------+
                                              |
                                              | fzf algo
                                              v
                                       FuzzyMatchV2
```

The frontend is responsible for:

- Collecting candidate items (files, applications, commands, etc.)
- Rendering the UI
- Handling focus and window behavior

fzf-filterd is responsible only for scoring and filtering text.

---

## RPC API

### SetList

Registers a list of candidate strings.

```json
{
  "method": "TestRPC.SetList",
  "params": { "List": ["item1", "item2"] }
}
```

### SetCommandList

Registers an additional list (e.g. commands).

```json
{
  "method": "TestRPC.SetCommandList",
  "params": { "List": ["cmd1", "cmd2"] }
}
```

### Filter

Filters all registered lists using the given pattern.

```json
{
  "method": "TestRPC.Filter",
  "params": { "Pattern": "abc" }
}
```

Response:

```json
{
  "Results": [
    {
      "Type": "list",
      "Text": "example",
      "Score": 123,
      "Pos": [0, 2, 3]
    }
  ]
}
```

---

## Platform

- Windows (Win32 Named Pipe)
- Go runtime

The current implementation targets Windows explicitly. Other IPC mechanisms may be added in the future.

---

## Non-Goals

- Rendering a UI
- Replacing fzf itself
- Providing a full launcher solution

fzf-filterd is intentionally minimal and focused.

---

## License

This project follows the license terms of fzf for the reused algorithm. See LICENSE for details.

---

## Acknowledgements

- fzf by junegunn
- Go standard library
- go-winio
