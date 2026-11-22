//go:build !desktop

package main

// Protocol and acquisition tests do not need an OS display or graphics driver.
// Distributable helpers are built with -tags desktop by build_helper.py.
func main() { helperMain() }
