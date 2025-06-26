// Downloads all the missing Pacific Notion (KEXP) podcasts for this month.
package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

var (
	outputDir        = flag.String("o", "./", "Output directory for the podcasts")
	usePreviousMonth = flag.Bool("previous-month", false, "Use the previous month instead of the current one")
	previousMonths   = flag.Uint("p", 0, "Number of previous months to go back")
	debug            = flag.Bool("debug", false, "Debug mode")
)

func main() {
	flag.Parse()

	// 1. Figure out which month and year to process
	month, year := currentMonthAndYear()
	month, year = adjustForPast(month, year)
	debugPrintf("Month: %d, year: %d\n", month, year)

	// 2. Get the list of podcasts for this month
	//    a. Get sundays
	//    b. Try to pick the correct URL for each sunday
	sundays := findSundays(month, year)
	if !*usePreviousMonth && *previousMonths == 0 {
		sundays = filterSundaysUntilToday(sundays)
	}
	formattedSundays := formatDays(sundays)
	debugPrintf("Sundays: %v\n", formattedSundays)

	var wg sync.WaitGroup
	validURLs := make(chan string)
	for _, v := range formattedSundays {
		wg.Add(1)
		go func(sunday string) {
			defer wg.Done()

			// find valid URL, does some magic, error-prone, requires constant updating
			link := tryFindURLForDateMysteriosNumber(sunday, 12)
			if len(link) > 0 {
				debugPrintf("Valid URL: %v\n", link)
			}

			// filter missing downloads based on files in outputDir
			if link != "" && isDownloadMissing(*outputDir, link) {
				validURLs <- link
				debugPrintf("Missing URL: %v\n", link)
			}
		}(v)
	}
	wg.Wait()
	close(validURLs)

	if len(validURLs) == 0 {
		fmt.Println("No missing podcasts")
		return
	}

	// 3. Download the podcasts that are missing
	for link := range validURLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			err := downloadFile(u, *outputDir)
			if err != nil {
				fmt.Println(err)
			}
		}(link)
	}
	wg.Wait()
}

func currentMonthAndYear() (time.Month, int) {
	t := time.Now()
	return t.Month(), t.Year()
}

func adjustForPast(curMonth time.Month, curYear int) (time.Month, int) {
	if !*usePreviousMonth && *previousMonths == 0 {
		return curMonth, curYear
	}

	monthsDelta := 0

	if *usePreviousMonth {
		monthsDelta = 1
	} else if *previousMonths != 0 {
		monthsDelta = int(math.Abs(float64(*previousMonths)))
	}

	curMonth -= time.Month(monthsDelta)
	if curMonth <= 0 {
		curMonth = 12
		curYear -= 1
	}

	return curMonth, curYear

}

func findSundays(month time.Month, year int) []time.Time {
	day := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
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

func formatDays(days []time.Time) []string {
	formatted := []string{}

	for _, day := range days {
		formatted = append(formatted, formatSunday(day))
	}

	return formatted
}

func formatSunday(sunday time.Time) string {
	return sunday.Format("20060102")
}

func makeURLForDate(date string, mysteriousNumber int) string {
	return fmt.Sprintf("https://kexp-archive.streamguys1.com/content/kexp/%s0550%02d-33-1962-pacific-notions.mp3", date, mysteriousNumber)
}

func tryFindURLForDateMysteriosNumber(date string, mysteriousNumber int) string {
	if mysteriousNumber < 0 {
		return ""
	}

	urlCandidate := makeURLForDate(date, mysteriousNumber)
	debugPrintf("Trying %s\n", urlCandidate)
	resp, err := http.Head(urlCandidate)

	if err == nil && resp.StatusCode == 200 {
		return urlCandidate
	}

	return tryFindURLForDateMysteriosNumber(date, mysteriousNumber-1)
}

func downloadFile(url string, outputDir string) error {
	errMsgFormat := "failed downloading %s: %s"

	fmt.Printf("Downloading %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf(errMsgFormat, url, err)
	}
	defer resp.Body.Close()

	outputPath := path.Join(outputDir, path.Base(url))

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf(errMsgFormat, url, err)
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf(errMsgFormat, url, err)
	}
	return nil
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

func isDownloadMissing(outputDir, link string) bool {
	outputPath := path.Join(outputDir, path.Base(link))
	_, err := os.Stat(outputPath)
	return os.IsNotExist(err) == true
}

func debugPrintln(a ...any) {
	if *debug {
		fmt.Println(a...)
	}
}

func debugPrintf(format string, a ...any) {
	if *debug {
		fmt.Printf(format, a...)
	}
}
