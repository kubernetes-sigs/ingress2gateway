# Copilot Instructions for ingress2gateway

## General Guidelines

- **Apply conventions only to changed code**: When making edits, apply these conventions and style rules only to the code being changed. Do not modify unrelated code just to fix style, unless explicitly requested.

## Go Documentation Conventions

- **Exported functions/structs**: Prefix the comment with the symbol name (standard Go convention)
  ```go
  // MyFunction does something useful.
  func MyFunction() {}
  ```

- **Unexported functions/structs**: Do NOT prefix the comment with the symbol name
  ```go
  // Does something internal.
  func myFunction() {}
  ```

## Code Style

- **Comments**: End Go comments with a period, but not for inline comments on the same line as code
  ```go
  // This is a properly formatted comment.
  x := 1 // No period for inline comments
  ```

- **Return statements**: Add an empty line before the final return statement in a function, but do NOT add empty lines before non-final (early) return statements
  ```go
  func example() error {
      if err := doSomething(); err != nil {
          return err // No empty line before early return
      }

      return nil // Empty line before final return
  }
  ```
