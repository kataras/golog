package main

import (
	"fmt"

	"github.com/kataras/golog"
)

func main() {
	// Create a main logger
	logger := golog.Default
	logger.SetLevel("info")

	fmt.Println("=== Testing Children Logger Memory Management ===")

	// Create some child loggers
	dbLogger := logger.Child("database")
	apiLogger := logger.Child("api")
	cacheLogger := logger.Child("cache")

	fmt.Printf("Initial child count: %d\n", logger.ChildCount())
	fmt.Printf("Child keys: %v\n", logger.ListChildKeys())

	// Test logging from children
	dbLogger.Info("Database connection established")
	apiLogger.Warn("API rate limit approaching")
	cacheLogger.Error("Cache miss detected")

	// Test removing a child
	fmt.Printf("\nRemoving 'api' child...\n")
	removed := logger.RemoveChild("api")
	fmt.Printf("Removed: %t\n", removed)
	fmt.Printf("Child count after removal: %d\n", logger.ChildCount())
	fmt.Printf("Child keys after removal: %v\n", logger.ListChildKeys())

	// Try to remove non-existent child
	fmt.Printf("\nTrying to remove non-existent child...\n")
	removed = logger.RemoveChild("nonexistent")
	fmt.Printf("Removed: %t\n", removed)

	// Create more children
	logger.Child("auth").Info("Authentication module loaded")
	logger.Child("metrics").Info("Metrics collection started")

	fmt.Printf("\nAfter adding more children:\n")
	fmt.Printf("Child count: %d\n", logger.ChildCount())
	fmt.Printf("Child keys: %v\n", logger.ListChildKeys())

	// Test clearing all children
	fmt.Printf("\nClearing all children...\n")
	logger.ClearChildren()
	fmt.Printf("Child count after clear: %d\n", logger.ChildCount())
	fmt.Printf("Child keys after clear: %v\n", logger.ListChildKeys())

	// Test that cleared children are truly gone
	fmt.Printf("\nTesting that children are truly cleared:\n")
	newDbLogger := logger.Child("database")
	newDbLogger.Info("New database logger created")

	fmt.Printf("Final child count: %d\n", logger.ChildCount())
	fmt.Printf("Final child keys: %v\n", logger.ListChildKeys())

	fmt.Println("\n=== Test Complete ===")
}
