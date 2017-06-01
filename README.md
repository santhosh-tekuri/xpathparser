# xpath

[![GoDoc](https://godoc.org/github.com/santhosh-tekuri/xpath?status.svg)](https://godoc.org/github.com/santhosh-tekuri/xpath)
[![Go Report Card](https://goreportcard.com/badge/github.com/santhosh-tekuri/xpath)](https://goreportcard.com/report/github.com/santhosh-tekuri/xpath)
[![Build Status](https://travis-ci.org/santhosh-tekuri/xpath.svg?branch=master)](https://travis-ci.org/santhosh-tekuri/xpath)
[![codecov.io](https://codecov.io/github/santhosh-tekuri/xpath/coverage.svg?branch=master)](https://codecov.io/github/santhosh-tekuri/xpath?branch=master)

Package xpath provides lexer and parser for XPath 1.0.

This Package parses given XPath expression to expression model. To Evaluate XPath, use https://github.com/santhosh-tekuri/xpatheng

## Example

```go
expr, err := xpath.Parse("(/a/b)[5]")
if err != nil {
  fmt.Println(err)
  return
}
fmt.Println(expr)
```
