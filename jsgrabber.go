package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var downloadCount int32

func runCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("[-] Command failed: %s %v", name, args)
	}
}

func runBashCommand(script string) {
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("[-] Command failed (continuing): %s\nError: %v", script, err)
	}
}

func checkTool(tool string) {
	_, err := exec.LookPath(tool)
	if err != nil {
		log.Fatalf("[-] Required tool not found: %s", tool)
	}
}

func deduplicateJSLinks(files []string, outputFile string) {
	seen := make(map[string]string)
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			log.Printf("[-] Could not open file %s: %v", file, err)
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if !strings.Contains(url, ".js") {
				continue
			}
			parts := strings.Split(url, "?")
			base := parts[0]
			if _, exists := seen[base]; !exists {
				seen[base] = url
			}
		}
		f.Close()
	}

	out, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("[-] Failed to create output file: %v", err)
	}
	defer out.Close()
	for _, fullURL := range seen {
		out.WriteString(fullURL + "\n")
	}
	fmt.Printf("[+] Wrote %d unique JS links (ignoring query params) to %s\n", len(seen), outputFile)
}

func countLines(filePath string) int {
	file, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	return lines
}

func downloadJSFilesConcurrently(urls []string, dir string, concurrency int) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	client := &http.Client{Timeout: 20 * time.Second}

	for i, url := range urls {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int, url string) {
			defer wg.Done()
			defer func() { <-sem }()

			name := filepath.Base(strings.Split(url, "?")[0])
			if name == "" {
				name = "script.js"
			}
			output := filepath.Join(dir, fmt.Sprintf("%d_%s", i+1, name))

			resp, err := client.Get(url)
			if err != nil {
				log.Printf("[-] Failed to download %s: %v", url, err)
				return
			}
			defer resp.Body.Close()

			outFile, err := os.Create(output)
			if err != nil {
				log.Printf("[-] Failed to create file %s: %v", output, err)
				return
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, resp.Body)
			if err != nil {
				log.Printf("[-] Error writing to file %s: %v", output, err)
				return
			}

			atomic.AddInt32(&downloadCount, 1)
		}(i, url)
	}
	wg.Wait()
	fmt.Printf("[+] Downloaded %d JS files.\n", downloadCount)
}

func main() {
	domain := flag.String("d", "", "Target domain (e.g., example.com)")
	autoDownload := flag.Bool("i", false, "Download JS files automatically without asking")
	runAnalyzers := flag.Bool("l", false, "Run LinkFinder and SecretFinder after downloading")
	flag.Parse()

	if *domain == "" {
		log.Fatalln("[-] Please provide a domain using -d flag.")
	}

	example := *domain
	dir := example + "_JSrecon"
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("[-] Failed to create directory: %v", err)
	}
	os.Chdir(dir)

	requiredTools := []string{"katana", "getJS", "gau", "waybackurls", "hakrawler", "curl", "js-beautify"}
	if *runAnalyzers {
		requiredTools = append(requiredTools, "linkfinder", "secretfinder")
	}
	for _, tool := range requiredTools {
		checkTool(tool)
	}

	fmt.Println("[+] Gathering JS files...")
	
	reconDataDir := "js_raw_sources"
err = os.MkdirAll(reconDataDir, 0755)
if err != nil {
	log.Fatalf("[-] Failed to create directory for JS gathering: %v", err)
}

	runBashCommand(fmt.Sprintf("katana -u https://%s -d 3 -silent | (grep -iE '.js(\\?|$)' || true) | sort -u > %s/%s_katanajs.txt", example, reconDataDir, example))
runBashCommand(fmt.Sprintf("getJS --url https://%s --complete > %s/%s_getjs.txt", example, reconDataDir, example))
runBashCommand(fmt.Sprintf("(gau %s | grep '.js' || true) >> %s/%s_gaujs.txt", example, reconDataDir, example))
runBashCommand(fmt.Sprintf("(waybackurls %s | grep '.js' || true) >> %s/%s_wbujs.txt", example, reconDataDir, example))
runBashCommand(fmt.Sprintf("echo https://%s | hakrawler -insecure -subs | (grep '.js' || true) | sort -u > %s/%s_hakrawlerjs.txt", example, reconDataDir, example))


	fmt.Println("[+] Combining and deduplicating JS links...")
	finalFile := example + "_final-js-links.txt"
	deduplicateJSLinks([]string{
    filepath.Join(reconDataDir, example+"_katanajs.txt"),
    filepath.Join(reconDataDir, example+"_getjs.txt"),
    filepath.Join(reconDataDir, example+"_gaujs.txt"),
    filepath.Join(reconDataDir, example+"_wbujs.txt"),
    filepath.Join(reconDataDir, example+"_hakrawlerjs.txt"),
}, finalFile)

	total := countLines(finalFile)
	if total == 0 {
		fmt.Println("[!] No JS files found. Exiting.")
		return
	}

	if *autoDownload {
		fmt.Println("[+] Downloading JS files...")
		downloadDir := "downloaded_jsfiles"
		err = os.MkdirAll(downloadDir, 0755)
		if err != nil {
			log.Fatalf("[-] Failed to create download directory: %v", err)
		}

		file, err := os.Open(finalFile)
		if err != nil {
			log.Fatalf("[-] Failed to open JS links file: %v", err)
		}
		defer file.Close()

		var urls []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url != "" {
				urls = append(urls, url)
			}
		}

		downloadJSFilesConcurrently(urls, downloadDir, 20)

		fmt.Println("[+] Beautifying JS files...")
		beautifyDir := "beautified_jsfiles"
		err = os.MkdirAll(beautifyDir, 0755)
		if err != nil {
			log.Fatalf("[-] Failed to create beautify directory: %v", err)
		}
		runBashCommand(fmt.Sprintf("count=0; for f in %s/*.js; do js-beautify \"$f\" -o \"%s/$(basename $f)\" >/dev/null 2>&1 && count=$((count+1)); done; echo \"[+] Beautified $count JS files.\"", downloadDir, beautifyDir))

		if *runAnalyzers {
			resultsDir := example + "_analysis_results"
			err = os.MkdirAll(resultsDir, 0755)
			if err != nil {
				log.Fatalf("[-] Failed to create analysis results directory: %v", err)
			}
			fmt.Println("[+] Running LinkFinder and SecretFinder...")
			runBashCommand(fmt.Sprintf("for f in %s/*.js; do [ -f \"$f\" ] && linkfinder -i \"$f\" -o cli; done | sort -u > %s/linkfinder_links.txt", beautifyDir, resultsDir))
			runBashCommand(fmt.Sprintf("for f in %s/*.js; do secretfinder -i \"$f\" -g file -o cli; done | grep -v 'file://' | sort -u > %s/all_secrets.txt", beautifyDir, resultsDir))
			fmt.Printf("[+] Analysis complete. Results saved in %s/\n", resultsDir)
		}
	}
}
