package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func runMemoryManager(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Memory Manager")
	fmt.Println("Commands: l/list, d <n>/del <n>, da/delall, q/quit")

	for {
		records, err := listStoredMemoryRecords()
		if err != nil {
			fmt.Printf("memory error: %v\n", err)
			return
		}

		if len(records) == 0 {
			fmt.Println("No stored memories found.")
			return
		}

		fmt.Println()
		for i, record := range records {
			fmt.Printf("%d. %s\n", i+1, record.Content)
		}

		fmt.Print("memory> ")
		if !scanner.Scan() {
			fmt.Println()
			return
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" || input == "l" || input == "list" {
			continue
		}

		fields := strings.Fields(input)
		cmd := strings.ToLower(fields[0])

		switch cmd {
		case "q", "quit", "exit":
			return
		case "d", "del", "delete":
			if len(fields) != 2 {
				fmt.Println("Usage: d <memory_number>")
				continue
			}
			n, err := strconv.Atoi(fields[1])
			if err != nil || n < 1 || n > len(records) {
				fmt.Println("Invalid memory number.")
				continue
			}
			if err := deleteMemoryByID(ctx, records[n-1].ID); err != nil {
				fmt.Printf("delete error: %v\n", err)
				continue
			}
			fmt.Printf("Deleted memory %d.\n", n)
		case "da", "delall", "deleteall":
			fmt.Print("Delete ALL memories? (yes/no): ")
			if !scanner.Scan() {
				fmt.Println()
				return
			}
			confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if confirm != "yes" && confirm != "y" {
				fmt.Println("Canceled.")
				continue
			}
			for _, record := range records {
				if err := deleteMemoryByID(ctx, record.ID); err != nil {
					fmt.Printf("delete error: %v\n", err)
					break
				}
			}
			fmt.Println("Deleted all listed memories.")
		default:
			fmt.Println("Unknown command. Use: l, d <n>, da, q")
		}
	}
}
