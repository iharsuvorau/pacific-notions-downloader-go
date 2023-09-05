// Downloads all the missing Pacific Notion (KEXP) podcasts for this month.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"
)

var outputDir = flag.String("o", "./", "Output directory for the podcasts")

func main() {
	flag.Parse()

	// 1. Get the current month
	month := getCurrentMonth()

	// 2. Get the list of podcasts for this month
	//    a. Get sundays
	//    b. Try to pick the correct URL for each sunday
	sundays := getSundaysForMonth(month)
	sundays = filterSundaysUntilToday(sundays)
	formattedSundays := formatDays(sundays)
	validURLs := []string{}
	for _, sunday := range formattedSundays {
		url := tryFindURLForDateMysteriosNumber(sunday, 9)
		if url != "" {
			validURLs = append(validURLs, url)
		}
	}

	// 3. Download the podcasts that are missing
	validURLs = filterMissingDownloads(*outputDir, validURLs)

	if len(validURLs) == 0 {
		fmt.Println("No missing podcasts")
		return
	}

	errors := make(chan error, len(validURLs))
	for _, url := range validURLs {
		go downloadFile(url, *outputDir, errors)
	}

	for i := 0; i < len(validURLs); i++ {
		err := <-errors
		if err != nil {
			fmt.Println(err)
		}
	}
}

func getCurrentMonth() time.Month {
	return time.Now().Month()
}

func getSundaysForMonth(month time.Month) []time.Time {
	day := time.Date(time.Now().Year(), month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := day.AddDate(0, 1, -1)
	sundays := []time.Time{}

	for {
		if day.Weekday() == time.Sunday {
			sundays = append(sundays, day)
		}
		if day == lastDay {
			break
		}
		day = day.AddDate(0, 0, 1)
	}

	return sundays
}

func filterSundaysUntilToday(sundays []time.Time) []time.Time {
	today := time.Now().Day()
	filtered := []time.Time{}

	for _, sunday := range sundays {
		if sunday.Day() <= today {
			filtered = append(filtered, sunday)
		}
	}

	return filtered
}

func formatSunday(sunday time.Time) string {
	return sunday.Format("20060102")
}

func formatDays(days []time.Time) []string {
	formatted := []string{}

	for _, day := range days {
		formatted = append(formatted, formatSunday(day))
	}

	return formatted
}

func makeURLForDate(date string, mysteriousNumber int) string {
	return fmt.Sprintf("https://kexp-archive.streamguys1.com/content/kexp/%s06000%d-33-515-pacific-notions.mp3", date, mysteriousNumber)
}

func tryFindURLForDateMysteriosNumber(date string, mysteriousNumber int) string {
	if mysteriousNumber < 0 {
		return ""
	}

	urlCandidate := makeURLForDate(date, mysteriousNumber)
	resp, err := http.Head(urlCandidate)

	if err == nil && resp.StatusCode == 200 {
		return urlCandidate
	}

	return tryFindURLForDateMysteriosNumber(date, mysteriousNumber-1)
}

func downloadFile(url string, outputDir string, ch chan<- error) {
	errMsgFormat := "failed downloading %s: %s"

	fmt.Printf("Downloading %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		err = fmt.Errorf(errMsgFormat, url, err)
		ch <- err
		return
	}
	defer resp.Body.Close()

	outputPath := path.Join(outputDir, path.Base(url))

	out, err := os.Create(outputPath)
	if err != nil {
		err = fmt.Errorf(errMsgFormat, url, err)
		ch <- err
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		err = fmt.Errorf(errMsgFormat, url, err)
	}
	ch <- err
}

func filterMissingDownloads(outputDir string, urls []string) []string {
	missing := []string{}

	for _, url := range urls {
		outputPath := path.Join(outputDir, path.Base(url))
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			missing = append(missing, url)
		}
	}

	return missing
}
