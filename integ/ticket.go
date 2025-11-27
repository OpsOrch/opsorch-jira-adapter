//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-jira-adapter/ticket"
)

func main() {
	// Test statistics
	var totalTests, passedTests, failedTests int
	startTime := time.Now()

	testResult := func(name string, err error) {
		totalTests++
		if err != nil {
			failedTests++
			log.Printf("❌ %s: %v", name, err)
		} else {
			passedTests++
			fmt.Printf("✅ %s passed\n", name)
		}
	}

	// Configuration from environment variables
	apiToken := os.Getenv("JIRA_API_TOKEN")
	apiURL := os.Getenv("JIRA_API_URL")
	projectKey := os.Getenv("JIRA_PROJECT_KEY")
	userEmail := os.Getenv("JIRA_USER_EMAIL")

	if apiToken == "" {
		log.Fatal("JIRA_API_TOKEN environment variable is required")
	}
	if apiURL == "" {
		log.Fatal("JIRA_API_URL environment variable is required")
	}
	if projectKey == "" {
		log.Fatal("JIRA_PROJECT_KEY environment variable is required")
	}
	if userEmail == "" {
		log.Fatal("JIRA_USER_EMAIL environment variable is required")
	}

	fmt.Println("=================================")
	fmt.Println("Jira Adapter Integration Test")
	fmt.Println("=================================")
	fmt.Printf("API URL: %s\n", apiURL)
	fmt.Printf("Project Key: %s\n", projectKey)
	fmt.Printf("Started: %s\n\n", startTime.Format("2006-01-02 15:04:05"))

	ctx := context.Background()

	// Create the ticket provider
	config := map[string]any{
		"apiToken":   apiToken,
		"apiURL":     apiURL,
		"projectKey": projectKey,
		"email":      userEmail,
		"source":     "jira-integ-test",
	}

	provider, err := ticket.New(config)
	if err != nil {
		log.Fatalf("Failed to create Jira provider: %v", err)
	}

	// Test 1: Query all tickets
	fmt.Println("\n=== Test 1: Query All Tickets ===")
	tickets, err := provider.Query(ctx, schema.TicketQuery{})
	if err != nil {
		testResult("Query all tickets", err)
	} else {
		fmt.Printf("Found %d tickets\n", len(tickets))
		for i, t := range tickets {
			if i >= 5 {
				fmt.Printf("... and %d more\n", len(tickets)-5)
				break
			}
			fmt.Printf("  [%d] Key: %s, Title: %s, Status: %s\n",
				i+1, t.Key, t.Title, t.Status)

			// Display metadata fields if present
			if priority, ok := t.Metadata["priority"].(string); ok {
				fmt.Printf("       Priority: %s\n", priority)
			}
			if issueType, ok := t.Metadata["issue_type"].(string); ok {
				fmt.Printf("       Issue Type: %s\n", issueType)
			}
			if labels, ok := t.Metadata["labels"].([]string); ok && len(labels) > 0 {
				fmt.Printf("       Labels: %v\n", labels)
			}
			if components, ok := t.Metadata["components"].([]string); ok && len(components) > 0 {
				fmt.Printf("       Components: %v\n", components)
			}

			// Validate required fields
			if t.ID == "" || t.Key == "" || t.Title == "" {
				testResult("Validate ticket fields", fmt.Errorf("missing required fields in ticket %d", i))
			}
		}
		testResult("Query all tickets", nil)
	}

	// Test 2: Create a new ticket
	fmt.Println("\n=== Test 2: Create New Ticket ===")
	newTicketInput := schema.CreateTicketInput{
		Title:       fmt.Sprintf("Integration Test Ticket %d", time.Now().Unix()),
		Description: "This is a test ticket created by the OpsOrch Jira adapter integration test.",
		Fields: map[string]any{
			"priority": "High",
			"labels":   []string{"integration-test", "opsorch"},
		},
	}

	newTicket, err := provider.Create(ctx, newTicketInput)
	if err != nil {
		testResult("Create ticket", err)
	} else {
		fmt.Printf("Successfully created ticket:\n")
		fmt.Printf("  ID: %s\n", newTicket.ID)
		fmt.Printf("  Key: %s\n", newTicket.Key)
		fmt.Printf("  Title: %s\n", newTicket.Title)
		fmt.Printf("  Status: %s\n", newTicket.Status)
		fmt.Printf("  Created: %s\n", newTicket.CreatedAt.Format("2006-01-02 15:04:05"))

		// Display metadata fields
		if priority, ok := newTicket.Metadata["priority"].(string); ok {
			fmt.Printf("  Priority: %s\n", priority)
		}
		if labels, ok := newTicket.Metadata["labels"].([]string); ok {
			fmt.Printf("  Labels: %v\n", labels)
		}

		if newTicket.Title != newTicketInput.Title {
			testResult("Validate created ticket title", fmt.Errorf("title mismatch"))
		} else {
			testResult("Create ticket", nil)
		}

		// Test 3: Get the created ticket
		fmt.Println("\n=== Test 3: Get Created Ticket ===")
		gotTicket, err := provider.Get(ctx, newTicket.Key)
		if err != nil {
			testResult("Get ticket", err)
		} else {
			if gotTicket.ID != newTicket.ID {
				testResult("Validate fetched ticket ID", fmt.Errorf("ID mismatch: expected %s, got %s", newTicket.ID, gotTicket.ID))
			} else {
				testResult("Get ticket", nil)
			}
		}

		// Test 4: Update the ticket
		fmt.Println("\n=== Test 4: Update Ticket ===")
		updatedTitle := newTicket.Title + " (Updated)"
		updatedDesc := "Updated description."

		updatedTicket, err := provider.Update(ctx, newTicket.Key, schema.UpdateTicketInput{
			Title:       &updatedTitle,
			Description: &updatedDesc,
		})
		if err != nil {
			testResult("Update ticket", err)
		} else {
			if updatedTicket.Title != updatedTitle {
				testResult("Validate updated title", fmt.Errorf("expected %s, got %s", updatedTitle, updatedTicket.Title))
			} else if updatedTicket.Description != updatedDesc {
				testResult("Validate updated description", fmt.Errorf("expected %s, got %s", updatedDesc, updatedTicket.Description))
			} else {
				testResult("Update ticket", nil)
			}
		}

		// Test 5: Query with text search
		fmt.Println("\n=== Test 5: Query with Text Search ===")
		// Search for the unique timestamp in our title
		uniquePart := strings.Split(newTicketInput.Title, "Ticket ")[1]
		searchResults, err := provider.Query(ctx, schema.TicketQuery{
			Query: uniquePart,
		})
		if err != nil {
			testResult("Query with text search", err)
		} else {
			found := false
			for _, t := range searchResults {
				if t.Key == newTicket.Key {
					found = true
					break
				}
			}
			if found {
				testResult("Query with text search", nil)
			} else {
				testResult("Query with text search", fmt.Errorf("created ticket not found in search results"))
			}
		}

		// Test 6: Cleanup - Transition to Done (if possible)
		// Note: This is tricky because transition names vary by project workflow.
		// We'll try common ones but won't fail the test if it doesn't work, just warn.
		fmt.Println("\n=== Test 6: Cleanup (Attempt to Close) ===")
		doneStatus := "Done" // Common status
		_, err = provider.Update(ctx, newTicket.Key, schema.UpdateTicketInput{
			Status: &doneStatus,
		})
		if err != nil {
			fmt.Printf("⚠️  Could not transition to 'Done': %v\n", err)
			fmt.Println("   (This is expected if your project workflow uses different status names)")
			// Try "Closed" as fallback
			closedStatus := "Closed"
			_, err = provider.Update(ctx, newTicket.Key, schema.UpdateTicketInput{
				Status: &closedStatus,
			})
			if err != nil {
				fmt.Printf("⚠️  Could not transition to 'Closed': %v\n", err)
			} else {
				fmt.Printf("✅ Transitioned to 'Closed'\n")
			}
		} else {
			fmt.Printf("✅ Transitioned to 'Done'\n")
		}
		// We don't count this as a pass/fail test because of workflow variability
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\n=================================")
	fmt.Println("Test Summary")
	fmt.Println("=================================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d ✅\n", passedTests)
	fmt.Printf("Failed: %d ❌\n", failedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)

	if failedTests == 0 {
		fmt.Println("\n✅ All tests passed successfully!")
	} else {
		fmt.Printf("\n⚠️  %d test(s) failed. Please review the output above.\n", failedTests)
		os.Exit(1)
	}
}
