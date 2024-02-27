package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	// Customize HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Open and defer close of essential files
	urlsFile, stringsFile, outputFile, err := openFiles()
	if err != nil {
		fmt.Println("Error opening files:", err)
		return
	}
	defer urlsFile.Close()
	defer stringsFile.Close()
	defer outputFile.Close()

	// Buffered writer for outputFile
	writer := bufio.NewWriter(outputFile)
	defer writer.Flush() // Ensure buffered writer flushes its content to disk at the end

	// Read strings into a slice
	strings, err := readStrings(stringsFile)
	if err != nil {
		fmt.Println("Error reading strings:", err)
		return
	}

	// Process URLs concurrently
	processUrls(urlsFile, client, strings, writer)
}

func openFiles() (*os.File, *os.File, *os.File, error) {
	urlsFile, err := os.Open("urls.txt")
	if err != nil {
		return nil, nil, nil, err
	}

	stringsFile, err := os.Open("strings.txt")
	if err != nil {
		urlsFile.Close() // Ensure previously opened files are closed on error
		return nil, nil, nil, err
	}

	outputFile, err := os.Create("output.txt")
	if err != nil {
		urlsFile.Close()
		stringsFile.Close()
		return nil, nil, nil, err
	}

	return urlsFile, stringsFile, outputFile, nil
}

func readStrings(stringsFile *os.File) ([]string, error) {
	var strings []string
	scanner := bufio.NewScanner(stringsFile)
	for scanner.Scan() {
		strings = append(strings, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return strings, nil
}

func processUrls(urlsFile *os.File, client *http.Client, strings []string, writer *bufio.Writer) {
	scanner := bufio.NewScanner(urlsFile)
	var wg sync.WaitGroup
	results := make(chan string)

	// Collect results and write to file
	go func() {
		for result := range results {
			_, err := writer.WriteString(result)
			if err != nil {
				fmt.Println("Error writing to output file:", err)
			}
		}
		writer.Flush() // Make sure to flush after all results are written
	}()

	for scanner.Scan() {
		url := scanner.Text()
		wg.Add(1)

		go func(url string) {
			defer wg.Done()

			resp, err := client.Get(url)
			if err != nil {
				fmt.Println("Error GET request:", err)
				return
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Error reading body:", err)
				return
			}

			for _, s := range strings {
				if bytes.Contains(body, []byte(s)) {
					result := fmt.Sprintf("String Found: %s - URL: %s\n", s, url)
					results <- result
				}
			}
		}(url)
	}

	wg.Wait()      // Wait for all URL processing goroutines to finish
	close(results) // Close the results channel to signal the writing goroutine to finish
}
