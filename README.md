# Keratin

[![Go Reference](https://pkg.go.dev/badge/github.com/gowool/keratin.svg)](https://pkg.go.dev/github.com/gowool/keratin)
[![Go Report Card](https://goreportcard.com/badge/github.com/gowool/keratin)](https://goreportcard.com/report/github.com/gowool/keratin)
[![codecov](https://codecov.io/github/gowool/keratin/graph/badge.svg?token=IBP5235ZZ4)](https://codecov.io/github/gowool/keratin)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://github.com/gowool/keratin/blob/main/LICENSE)

## Overview

`Keratin` is a thin, idiomatic Go wrapper around `http.ServeMux` that provides a clean, composable API for building HTTP servers with route grouping, middleware support, and error handling—all while maintaining zero dependencies beyond the Go standard library and minimal external packages for testing and UUID generation.

### Key Features

- **Route Grouping**: Organize routes into logical groups with shared prefixes and middleware
- **Flexible Middleware**: Apply middleware at router, group, or route level with priority-based execution
- **Pre-Middleware**: Register middleware that executes before route matching
- **Error Handling**: Centralized error handling with custom error handlers
- **Type-Safe**: Built-in types for handlers, middleware, and error handlers
- **Method Shorthands**: Convenience methods for all HTTP verbs (GET, POST, PUT, DELETE, PATCH, etc.)
- **Zero Abstraction Leak**: Leverages `http.ServeMux` pattern matching without re-implementing routing logic
- **Minimal Dependencies**: Only uses `github.com/google/uuid` for middleware IDs and `github.com/stretchr/testify` for testing

### Why `Keratin`?

Just as keratin provides the essential structure that makes wool strong, flexible, and resilient, `Keratin` provides the essential structure for building robust HTTP services in the Gowool ecosystem. While Go's `http.ServeMux` is powerful, it lacks built-in support for route grouping and middleware at the router level. `Keratin` fills this gap by providing a thin, composable wrapper that:

- Maintains the simplicity and performance of the standard library
- Adds essential features for building production-grade HTTP services
- Follows idiomatic Go patterns and conventions
- Provides a clean, fluent API for route definition

## Installation

```bash
go get github.com/gowool/keratin
```

## Quick Start

```go
package main

import (
	"log/slog"
    "net/http"

    "github.com/gowool/keratin"
)

func main() {
    router := keratin.NewRouter(keratin.ErrorHandlerFunc(func(w http.ResponseWriter, _ *http.Request, err error) {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }))

    router.GET("/health", keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("OK"))
        return nil
    }))

    api := router.Group("/api")
    api.UseFunc(loggingMiddleware)

    v1 := api.Group("/v1")
    v1.GET("/users", listUsers())
    v1.POST("/users", createUser())

    _ = http.ListenAndServe(":8080", router)
}

func loggingMiddleware(next keratin.Handler) keratin.Handler {
    return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		defer slog.Info("request",
			slog.String("method", r.Method),
			slog.String("protocol", r.Proto),
			slog.String("host", r.Host),
			slog.String("pattern", r.Pattern),
			slog.String("uri", r.RequestURI),
			slog.String("path", r.URL.Path),
			slog.String("referer", r.Referer()),
			slog.String("user_agent", r.UserAgent()),
        )
		
        return next.ServeHTTP(w, r)
    })
}

func listUsers() keratin.Handler {
	return keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("users"))
		return nil
	})
}

func createUser() keratin.Handler {
	return keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusCreated)
		return nil
	})
}
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

