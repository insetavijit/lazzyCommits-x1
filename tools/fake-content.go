package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

var words = []string{"alpha", "beta", "gamma", "delta", "omega", "zenith", "quantum", "cipher", "daemon", "flux"}

func randomString(n int) string {
	b := make([]string, n)
	for i := range b {
		b[i] = words[rand.Intn(len(words))]
	}
	return fmt.Sprintf("%v", b)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	sandboxPath := "./Sandbox"
	entries, err := os.ReadDir(sandboxPath)
	if err != nil {
		fmt.Printf("Error reading Sandbox: %v\n", err)
		return
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(sandboxPath, entry.Name()))
		}
	}

	if len(dirs) == 0 {
		fmt.Println("No directories found in Sandbox.")
		return
	}

	// Pick 3 random directories (or all if less than 3)
	rand.Shuffle(len(dirs), func(i, j int) { dirs[i], dirs[j] = dirs[j], dirs[i] })
	count := 3
	if len(dirs) < count {
		count = len(dirs)
	}

	for i := 0; i < count; i++ {
		fileName := fmt.Sprintf("test-%s-%d.txt", words[rand.Intn(len(words))], rand.Intn(1000))
		filePath := filepath.Join(dirs[i], fileName)
		content := fmt.Sprintf("Random content: %s\nGenerated at: %s\n", randomString(5), time.Now().Format(time.RFC3339))

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			fmt.Printf("Error writing to %s: %v\n", dirs[i], err)
			continue
		}
		fmt.Printf("Created fake file: %s\n", filePath)
	}
}
