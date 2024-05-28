// chartRevenue: Test Botify APIs
// Analysis based on 1MM URL maximum
// Written by Jason Vicinanza

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DateRanges struct used to hold the monthly date ranges and the YTD date range
// Used for revenue and visits data
type DateRanges struct {
	MonthlyRanges [][2]time.Time
	YTDRange      [2]time.Time
}

// monthDate structs used to store string values of the calculated start/end month dates for BQL use
type monthDates struct {
	StartMthDate string
	EndMthDate   string
}

// Slice used to store the name of the month
var startMthNames []string

// Define the slice to store startMthDate and endMthDate separately
var startMthDates = make([]string, 0)
var endMthDates = make([]string, 0)

// Slices used to store the SEO metrics
var seoMetricsTransactions []int
var seoMetricsRevenue []int
var seoMetricsVisits []int
var seoTransactionValue []int
var seoVisitValue []int

// Used to identify which analytics tool is in use
type AnalyticsID struct {
	ID string `json:"id"`
}

type transRevID struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Multiple bool          `json:"multiple"`
	Fields   []field       `json:"fields"`
	Groups   []interface{} `json:"groups"`
	Category []interface{} `json:"category"`
}

type field struct {
	ID string `json:"id"`
}

type meta struct {
	RequiredFields []string `json:"required_fields"`
}

// Used to store the revenue, transactions and visits
type Result struct {
	Dimensions []interface{} `json:"dimensions"`
	Metrics    []float64     `json:"metrics"`
}

type Response struct {
	Results []Result `json:"results"`
}

// Project URL
var projectURL = ""

// Version
var version = "v0.1"

// Colours
var purple = "\033[0;35m"
var green = "\033[0;32m"
var red = "\033[0;31m"
var bold = "\033[1m"
var reset = "\033[0m"
var checkmark = "\u2713"

// Specify your Botify API token here
var botify_api_token = "c1e6c5ab4a8dc6a16620fd0a885dd4bee7647205"

// Strings used to store the project credentials for API access
var orgName string
var projectName string

// Strings used to store the input project credentials
var orgNameInput string
var projectNameInput string

// Boolean to signal if the project credentials have been entered by the user
var credentialsInput = false

func main() {

	clearScreen()

	displayBanner()

	// Get the credentials if they have not been specified on the command line
	checkCredentials()

	// If the credentials have been provided on the command line use them
	if !credentialsInput {
		orgName = os.Args[1]
		projectName = os.Args[2]
	} else {
		orgName = orgNameInput
		projectName = projectNameInput
	}

	fmt.Println(bold+"\nOrganisation name:", orgName)
	fmt.Println(bold+"Project name:", projectName+reset)
	fmt.Println()

	// Generate the link to the project
	projectURL = "https://app.botify.com/" + orgName + "/" + projectName

	displaySeparator()

	// Revenue for the last 12 months
	seoRevenue()

	// Revenue & visits bar chart
	barChartRevenueVisits()

	// No. of transactions bar chart
	barChartTransactions()

	// Transaction value barchart
	barChartTransactionValue()

	// Theme river
	//themeRiverTime()

	displaySeparator()

	chartRevenueDone()
}

// Check that the org and project names have been specified as command line arguments
// if not prompt for them
// Pressing Enter exits
func checkCredentials() {

	if len(os.Args) < 3 {

		credentialsInput = true

		fmt.Print("\nEnter your project credentials. Press" + green + " Enter " + reset + "to exit chartRevenue" +
			"\n")

		fmt.Print(purple + "\nEnter organisation name: " + reset)
		fmt.Scanln(&orgNameInput)
		// Check if input is empty if so exit
		if strings.TrimSpace(orgNameInput) == "" {
			fmt.Println(green + "\nThank you for using chartRevenue. Goodbye!\n")
			os.Exit(0)
		}

		fmt.Print(purple + "Enter project name: " + reset)
		fmt.Scanln(&projectNameInput)
		// Check if input is empty if so exit
		if strings.TrimSpace(projectNameInput) == "" {
			fmt.Println(green + "\nThank you for using chartRevenue. Goodbye!\n")
			os.Exit(0)
		}
	}
}

func seoRevenue() {

	fmt.Println(purple + bold + "\nGetting revenue insights" + reset)

	// Get the date ranges
	dateRanges := calculateDateRanges()

	// Identify which analytics tool is used
	analyticsID := getAnalyticsID()
	fmt.Println("Analytics identified:", analyticsID)

	// Prepare the monthly dates ranges ready for use in the BQL
	// Populate the slice with string versions of the date ready for use in the BQL
	for _, dateRange := range dateRanges.MonthlyRanges {
		startMthDate := dateRange[0].Format("20060102")
		endMthDate := dateRange[1].Format("20060102")
		startMthDates = append(startMthDates, startMthDate)
		endMthDates = append(endMthDates, endMthDate)

		// Get the month name
		startDate, _ := time.Parse("20060102", startMthDate)
		startMthName := startDate.Format("January 2006")
		startMthNames = append(startMthNames, startMthName)
	}

	// Format the YTD range ready for use in the BQL
	startYTDDate := dateRanges.YTDRange[0].Format("20060102")
	endYTDDate := dateRanges.YTDRange[1].Format("20060102")

	// Get the revenue data
	getRevenueData(analyticsID, startYTDDate, endYTDDate, startMthDates, endMthDates)
}

// Get the revenue, transactions and visits data
func getRevenueData(analyticsID string, startYTDDate string, endYTDDate string, startMthDates []string, endMthDates []string) {

	var ytdMetricsTransactions = 0
	var ytdMetricsRevenue = 0
	var ytdMetricsVisits = 0
	var avgTransactionValue = 0
	var avgVisitValue = 0

	// Get monthly insights
	fmt.Println(bold + "\nMonthly organic insights" + reset)
	for i := range startMthDates {
		ytdMetricsTransactions, ytdMetricsRevenue, ytdMetricsVisits, avgTransactionValue, avgVisitValue = executeRevenueBQL(analyticsID, startMthDates[i], endMthDates[i])

		// Display the metrics (formatted)
		fmt.Printf(green+"Start: %s End: %s\n"+reset, startMthDates[i], endMthDates[i])
		formattedTransactions := formatWithCommas(ytdMetricsTransactions)
		fmt.Println("No. transactions", formattedTransactions)
		formattedRevenue := formatWithCommas(ytdMetricsRevenue)
		fmt.Println("Total revenue", formattedRevenue)
		fmt.Println("Average transaction value", avgTransactionValue)
		formattedVisits := formatWithCommas(ytdMetricsVisits)
		fmt.Println("No. of visits", formattedVisits)
		fmt.Println("Average visit value", avgVisitValue)
		fmt.Println("\n")

		// Append the metrics to the slices
		seoMetricsTransactions = append(seoMetricsTransactions, ytdMetricsTransactions)
		seoMetricsRevenue = append(seoMetricsRevenue, ytdMetricsRevenue)
		seoTransactionValue = append(seoTransactionValue, avgTransactionValue)
		seoMetricsVisits = append(seoMetricsVisits, ytdMetricsVisits)
	}
}

// Get the analytics ID
func getAnalyticsID() string {
	// First identify which analytics tool is integrated
	urlAPIAnalyticsID := "https://api.botify.com/v1/projects/" + orgName + "/" + projectName + "/collections"
	//fmt.Println(bold+"\nAnalytics ID end point:"+reset, urlAPIAnalyticsID)
	req, errorCheck := http.NewRequest("GET", urlAPIAnalyticsID, nil)

	// Define the headers
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "token "+botify_api_token)
	req.Header.Add("Content-Type", "application/json")

	if errorCheck != nil {
		log.Fatal(red+"\nError. getAnalyticsID. Cannot create request: "+reset, errorCheck)
		os.Exit(1)
	}
	// Create HTTP client and execute the request
	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	resp, errorCheck := client.Do(req)
	if errorCheck != nil {
		log.Fatal("Error. getAnalyticsID. Error: ", errorCheck)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read the response body
	responseData, errorCheck := ioutil.ReadAll(resp.Body)
	if errorCheck != nil {
		log.Fatal(red+"Error. getAnalyticsID. Cannot read response body: "+reset, errorCheck)
		os.Exit(1)
	}

	// Unmarshal the JSON data into the struct
	var analyticsIDs []AnalyticsID
	if err := json.Unmarshal(responseData, &analyticsIDs); err != nil {
		log.Fatal(red+"Error. getAnalyticsID. Cannot unmarshall the JSON: "+reset, err)
		os.Exit(1)
	}

	// Find and print the name value when the ID contains the word "visit"
	// Assume the first instance of "visit" contains the analytics ID
	for _, analyticsID := range analyticsIDs {
		if strings.Contains(analyticsID.ID, "visit") {
			return analyticsID.ID
		}
	}
	return "noAnalyticsdFound"
}

// Get the date ranges for the revenue and visits
func calculateDateRanges() DateRanges {
	currentTime := time.Now()
	dateRanges := make([][2]time.Time, 12)

	// Calculate the YTD date range
	year, _, _ := currentTime.Date()
	loc := currentTime.Location()
	startOfYear := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	endOfYTD := currentTime
	yearToDateRange := [2]time.Time{startOfYear, endOfYTD}

	// Calculate the date ranges for the last 12 months
	for i := 0; i < 12; i++ {
		// Calculate the start and end dates for the current range
		year, month, _ := currentTime.Date()
		loc := currentTime.Location()

		// Start of the current month range
		startDate := time.Date(year, month, 1, 0, 0, 0, 0, loc)

		var endDate time.Time
		if i == 0 {
			// End of the current month range (up to the current date)
			endDate = currentTime
		} else {
			// End of the previous month range
			endDate = startDate.AddDate(0, 1, -1)
		}

		// Store the range
		dateRanges[11-i] = [2]time.Time{startDate, endDate}

		// Move to the previous month
		currentTime = startDate.AddDate(0, -1, 0)
	}

	return DateRanges{MonthlyRanges: dateRanges, YTDRange: yearToDateRange}
}

// Execute the BQL for the specified date range
// func executeRevenueBQL(analyticsID string, startDate string, endDate string) {
func executeRevenueBQL(analyticsID string, startDate string, endDate string) (int, int, int, int, int) {

	// Get the revenue, no. transactions and visits - YTD
	bqlRevTrans := fmt.Sprintf(`
	{
    "collections": [
                    "conversion.dip",
                    "%s"
    ],
    "periods": [
        [
                    "%s",
                    "%s"
        ]
    ],
    "query": {
        "dimensions": [],
        "metrics": [
                    "conversion.dip.period_0.transactions",
                    "conversion.dip.period_0.revenue",    
                    "visits.dip.period_0.nb"
        ],
        "filters": {
            "and": [
                {
                    "field": "conversion.dip.period_0.medium",
                    "predicate": "eq",
                    "value": "organic"
                },
                {
                    "field": "visits.dip.period_0.medium",
                    "predicate": "eq",
                    "value": "organic"
           	     }
      	      ]
    	    }
 	   }
	}`, analyticsID, startDate, endDate)

	// Define the URL
	url := fmt.Sprintf("https://api.botify.com/v1/projects/%s/%s/query", orgName, projectName)

	// GET the HTTP request
	req, errorCheck := http.NewRequest("GET", url, nil)
	if errorCheck != nil {
		log.Fatal(red+"\nError. executeRevenueBQL. Cannot create request. Perhaps the provided credentials are invalid: "+reset, errorCheck)
	}

	// Define the body
	httpBody := []byte(bqlRevTrans)

	// Create the POST request
	req, errorCheck = http.NewRequest("POST", url, bytes.NewBuffer(httpBody))
	if errorCheck != nil {
		log.Fatal("Error. executeRevenueBQL. Cannot create request. Perhaps the provided credentials are invalid: ", errorCheck)
	}

	// Define the headers
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "token "+botify_api_token)
	req.Header.Add("Content-Type", "application/json")

	// Create HTTP client and execute the request
	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	resp, errorCheck := client.Do(req)
	if errorCheck != nil {
		log.Fatal("Error. executeRevenueBQL. Error: ", errorCheck)
	}
	defer resp.Body.Close()

	// Read the response body
	responseData, errorCheck := ioutil.ReadAll(resp.Body)
	if errorCheck != nil {
		log.Fatal(red+"Error. executeRevenueBQL. Cannot read response body: "+reset, errorCheck)
		os.Exit(1)
	}

	// Unmarshal the JSON data into the struct
	var response Response
	err := json.Unmarshal(responseData, &response)
	if err != nil {
		log.Fatalf("Error. executeRevenueBQL. Cannot unmarshal the JSON: %v", err)
	}

	var ytdMetricsTransactions = 0
	var ytdMetricsRevenue = 0
	var ytdMetricsVisits = 0
	var avgTransactionValue = 0
	var avgVisitValue = 0

	// Check if any data has been returned from the API. Count the number of elements in the response.Results slice
	responseCount := len(response.Results)

	if responseCount == 0 {
		fmt.Println(red + "Error. executeRevenueBQL. Cannot get Revenue & Visits data. Ensure the selected project is using GA4." + reset)
	} else {
		ytdMetricsTransactions = int(response.Results[0].Metrics[0])
		ytdMetricsRevenue = int(response.Results[0].Metrics[1])
		ytdMetricsVisits = int(response.Results[0].Metrics[2])
		// Compute the average transaction value
		avgTransactionValue = ytdMetricsRevenue / ytdMetricsTransactions
		avgVisitValue = ytdMetricsRevenue / ytdMetricsVisits
	}
	return ytdMetricsTransactions, ytdMetricsRevenue, ytdMetricsVisits, avgTransactionValue, avgVisitValue
}

// Bar chart. Revenue and Visits// Bar chart. No. of transactions
func barChartRevenueVisits() {

	//fmt.Println("seoMetricsTransactions:", seoMetricsTransactions)
	//fmt.Println("seoMetricsRevenue:", seoMetricsRevenue)
	//fmt.Println("seoMetricsVisits:", seoMetricsVisits)
	//fmt.Println("seoTransactionValue:", seoTransactionValue)
	//fmt.Println("startdates:", startMthDates)
	//fmt.Println("enddates:", endMthDates)
	//fmt.Println("month names:", startMthNames)

	// create a new bar instance
	bar := charts.NewBar()
	// set some global options like Title/Legend/ToolTip or anything else

	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title:    projectURL,
		Subtitle: "Revenue & visits",
		Link:     projectURL,
	}),
		charts.WithLegendOpts(opts.Legend{Right: "80px"}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 1,
			End:   100,
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1200px",
			Height: "600px",
		}),
		charts.WithColorsOpts(opts.Colors{"teal", "gray"}),
	)

	barDataRevenue := generateBarItems(seoMetricsRevenue)
	barDataVisits := generateBarItems(seoMetricsVisits)

	bar.SetXAxis(startMthNames).
		AddSeries("Revenue", barDataRevenue, charts.WithMarkPointNameCoordItemOpts(opts.MarkPointNameCoordItem{
			Name:       "special mark",
			Coordinate: []interface{}{"March", 100},
		})).
		AddSeries("Visits", barDataVisits).
		SetSeriesOptions(charts.WithMarkLineNameTypeItemOpts(
			opts.MarkLineNameTypeItem{Name: "Maximum", Type: "max"},
			opts.MarkLineNameTypeItem{Name: "Avg", Type: "average"},
		))

	//AddSeries("Transactions", barDataTransactions)

	// Where the magic happens
	f, _ := os.Create("seoRevenueVisitsBar.html")
	bar.Render(f)
}

// Bar chart. Visit value
func barChartVisitValue() {
	// create a new bar instance
	bar := charts.NewBar()

	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title:    projectURL,
		Subtitle: "Visit Value",
		Link:     projectURL,
	}),
		charts.WithLegendOpts(opts.Legend{Right: "80px"}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 1,
			End:   100,
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1200px",
			Height: "600px",
		}),
		charts.WithColorsOpts(opts.Colors{"red"}),
	)

	barDataVisitValue := generateBarItems(seoVisitValue)

	bar.SetXAxis(startMthNames).
		AddSeries("Transaction value", barDataVisitValue, charts.WithMarkPointNameCoordItemOpts(opts.MarkPointNameCoordItem{
			Name:       "special mark",
			Coordinate: []interface{}{"March", 100},
		})).
		SetSeriesOptions(charts.WithMarkLineNameTypeItemOpts(
			opts.MarkLineNameTypeItem{Name: "Maximum", Type: "max"},
			opts.MarkLineNameTypeItem{Name: "Avg", Type: "average"},
		))

	// Where the magic happens
	f, _ := os.Create("seoVisitValueBar.html")
	bar.Render(f)
}

// Bar chart. No. of transactions
func barChartTransactions() {
	// create a new bar instance
	bar := charts.NewBar()

	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title:    projectURL,
		Subtitle: "No. of Transactions",
		Link:     projectURL,
	}),
		charts.WithLegendOpts(opts.Legend{Right: "80px"}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 1,
			End:   100,
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1200px",
			Height: "600px",
		}),
		charts.WithColorsOpts(opts.Colors{"green"}),
	)

	barDataTransactions := generateBarItems(seoMetricsTransactions)

	bar.SetXAxis(startMthNames).
		AddSeries("Transactions", barDataTransactions, charts.WithMarkPointNameCoordItemOpts(opts.MarkPointNameCoordItem{
			Name:       "special mark",
			Coordinate: []interface{}{"March", 100},
		})).
		SetSeriesOptions(charts.WithMarkLineNameTypeItemOpts(
			opts.MarkLineNameTypeItem{Name: "Maximum", Type: "max"},
			opts.MarkLineNameTypeItem{Name: "Avg", Type: "average"},
		))

	// Where the magic happens
	f, _ := os.Create("seoTransactionsBar.html")
	bar.Render(f)
}

// Bar chart. No. of transactions
func barChartTransactionValue() {
	// create a new bar instance
	bar := charts.NewBar()
	// set some global options like Title/Legend/ToolTip or anything else

	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title:    projectURL,
		Subtitle: "Transaction Value",
		Link:     projectURL,
	}),
		charts.WithLegendOpts(opts.Legend{Right: "80px"}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "slider",
			Start: 1,
			End:   100,
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1200px",
			Height: "600px",
		}),
		charts.WithColorsOpts(opts.Colors{"blue"}),
	)

	barDataTransactionValue := generateBarItems(seoTransactionValue)

	bar.SetXAxis(startMthNames).
		AddSeries("Transaction value", barDataTransactionValue, charts.WithMarkPointNameCoordItemOpts(opts.MarkPointNameCoordItem{
			Name:       "special mark",
			Coordinate: []interface{}{"March", 100},
		})).
		SetSeriesOptions(charts.WithMarkLineNameTypeItemOpts(
			opts.MarkLineNameTypeItem{Name: "Maximum", Type: "max"},
			opts.MarkLineNameTypeItem{Name: "Avg", Type: "average"},
		))

	// Where the magic happens
	f, _ := os.Create("seoTransactionValueBar.html")
	bar.Render(f)
}

// Function to generate BarData items from an array of integers
func generateBarItems(revenue []int) []opts.BarData {
	items := make([]opts.BarData, len(revenue))
	for i, val := range revenue {
		items[i] = opts.BarData{Value: val}
	}
	return items
}

// Theme river chart
func themeRiverTime() *charts.ThemeRiver {
	tr := charts.NewThemeRiver()
	tr.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "ThemeRiver-SingleAxis-Time",
		}),
		charts.WithSingleAxisOpts(opts.SingleAxis{
			Type:   "time",
			Bottom: "10%",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger: "axis",
		}),
	)

	data := []opts.ThemeRiverData{
		{"2015/11/08", 10, "DQ"},
		{"2015/11/09", 15, "DQ"},
		{"2015/11/10", 35, "DQ"},
		{"2015/11/11", 38, "DQ"},
		{"2015/11/12", 22, "DQ"},
		{"2015/11/13", 16, "DQ"},
		{"2015/11/14", 7, "DQ"},
		{"2015/11/15", 2, "DQ"},
		{"2015/11/16", 17, "DQ"},
		{"2015/11/17", 33, "DQ"},
		{"2015/11/18", 40, "DQ"},
		{"2015/11/19", 32, "DQ"},
		{"2015/11/20", 26, "DQ"},
		{"2015/11/21", 35, "DQ"},
		{"2015/11/22", 40, "DQ"},
		{"2015/11/23", 32, "DQ"},
		{"2015/11/24", 26, "DQ"},
		{"2015/11/25", 22, "DQ"},
		{"2015/11/26", 16, "DQ"},
		{"2015/11/27", 22, "DQ"},
		{"2015/11/28", 10, "DQ"},
		{"2015/11/08", 35, "TY"},
		{"2015/11/09", 36, "TY"},
		{"2015/11/10", 37, "TY"},
		{"2015/11/11", 22, "TY"},
		{"2015/11/12", 24, "TY"},
		{"2015/11/13", 26, "TY"},
		{"2015/11/14", 34, "TY"},
		{"2015/11/15", 21, "TY"},
		{"2015/11/16", 18, "TY"},
		{"2015/11/17", 45, "TY"},
		{"2015/11/18", 32, "TY"},
		{"2015/11/19", 35, "TY"},
		{"2015/11/20", 30, "TY"},
		{"2015/11/21", 28, "TY"},
		{"2015/11/22", 27, "TY"},
		{"2015/11/23", 26, "TY"},
		{"2015/11/24", 15, "TY"},
		{"2015/11/25", 30, "TY"},
		{"2015/11/26", 35, "TY"},
		{"2015/11/27", 42, "TY"},
		{"2015/11/28", 42, "TY"},
		{"2015/11/08", 21, "SS"},
		{"2015/11/09", 25, "SS"},
		{"2015/11/10", 27, "SS"},
		{"2015/11/11", 23, "SS"},
		{"2015/11/12", 24, "SS"},
		{"2015/11/13", 21, "SS"},
		{"2015/11/14", 35, "SS"},
		{"2015/11/15", 39, "SS"},
		{"2015/11/16", 40, "SS"},
		{"2015/11/17", 36, "SS"},
		{"2015/11/18", 33, "SS"},
		{"2015/11/19", 43, "SS"},
		{"2015/11/20", 40, "SS"},
		{"2015/11/21", 34, "SS"},
		{"2015/11/22", 28, "SS"},
		{"2015/11/23", 26, "SS"},
		{"2015/11/24", 37, "SS"},
		{"2015/11/25", 41, "SS"},
		{"2015/11/26", 46, "SS"},
		{"2015/11/27", 47, "SS"},
		{"2015/11/28", 41, "SS"},
		{"2015/11/08", 10, "QG"},
		{"2015/11/09", 15, "QG"},
		{"2015/11/10", 35, "QG"},
		{"2015/11/11", 38, "QG"},
		{"2015/11/12", 22, "QG"},
		{"2015/11/13", 16, "QG"},
		{"2015/11/14", 7, "QG"},
		{"2015/11/15", 2, "QG"},
		{"2015/11/16", 17, "QG"},
		{"2015/11/17", 33, "QG"},
		{"2015/11/18", 40, "QG"},
		{"2015/11/19", 32, "QG"},
		{"2015/11/20", 26, "QG"},
		{"2015/11/21", 35, "QG"},
		{"2015/11/22", 40, "QG"},
		{"2015/11/23", 32, "QG"},
		{"2015/11/24", 26, "QG"},
		{"2015/11/25", 22, "QG"},
		{"2015/11/26", 16, "QG"},
		{"2015/11/27", 22, "QG"},
		{"2015/11/28", 10, "QG"},
		{"2015/11/08", 10, "SY"},
		{"2015/11/09", 15, "SY"},
		{"2015/11/10", 35, "SY"},
		{"2015/11/11", 38, "SY"},
		{"2015/11/12", 22, "SY"},
		{"2015/11/13", 16, "SY"},
		{"2015/11/14", 7, "SY"},
		{"2015/11/15", 2, "SY"},
		{"2015/11/16", 17, "SY"},
		{"2015/11/17", 33, "SY"},
		{"2015/11/18", 40, "SY"},
		{"2015/11/19", 32, "SY"},
		{"2015/11/20", 26, "SY"},
		{"2015/11/21", 35, "SY"},
		{"2015/11/22", 4, "SY"},
		{"2015/11/23", 32, "SY"},
		{"2015/11/24", 26, "SY"},
		{"2015/11/25", 22, "SY"},
		{"2015/11/26", 16, "SY"},
		{"2015/11/27", 22, "SY"},
		{"2015/11/28", 10, "SY"},
		{"2015/11/08", 10, "DD"},
		{"2015/11/09", 15, "DD"},
		{"2015/11/10", 35, "DD"},
		{"2015/11/11", 38, "DD"},
		{"2015/11/12", 22, "DD"},
		{"2015/11/13", 16, "DD"},
		{"2015/11/14", 7, "DD"},
		{"2015/11/15", 2, "DD"},
		{"2015/11/16", 17, "DD"},
		{"2015/11/17", 33, "DD"},
		{"2015/11/18", 4, "DD"},
		{"2015/11/19", 32, "DD"},
		{"2015/11/20", 26, "DD"},
		{"2015/11/21", 35, "DD"},
		{"2015/11/22", 40, "DD"},
		{"2015/11/23", 32, "DD"},
		{"2015/11/24", 26, "DD"},
		{"2015/11/25", 22, "DD"},
		{"2015/11/26", 16, "DD"},
		{"2015/11/27", 22, "DD"},
		{"2015/11/28", 10, "DD"},
	}

	tr.AddSeries("themeRiver", data)
	return tr
}

type ThemeriverExamples struct{}

func (ThemeriverExamples) Examples() {
	page := components.NewPage()
	page.AddCharts(
		themeRiverTime(),
	)

	f, err := os.Create("themeriver.html")
	if err != nil {
		panic(err)
	}
	page.Render(io.MultiWriter(f))
}

func chartRevenueDone() {
	// We're done
	fmt.Println(purple + "\nchartRevenue: Done!\n")
	fmt.Println(bold + green + "\nPress any key to exit..." + reset)
	var input string
	fmt.Scanln(&input)
	os.Exit(0)
}

// Display the welcome banner
func displayBanner() {
	//Banner
	//https://patorjk.com/software/taag/#p=display&c=bash&f=ANSI%20Shadow&t=SegmentifyLite
	fmt.Println(green + `


 ██████╗██╗  ██╗ █████╗ ██████╗ ████████╗██████╗ ███████╗██╗   ██╗███████╗███╗   ██╗██╗   ██╗███████╗
██╔════╝██║  ██║██╔══██╗██╔══██╗╚══██╔══╝██╔══██╗██╔════╝██║   ██║██╔════╝████╗  ██║██║   ██║██╔════╝
██║     ███████║███████║██████╔╝   ██║   ██████╔╝█████╗  ██║   ██║█████╗  ██╔██╗ ██║██║   ██║█████╗  
██║     ██╔══██║██╔══██║██╔══██╗   ██║   ██╔══██╗██╔══╝  ╚██╗ ██╔╝██╔══╝  ██║╚██╗██║██║   ██║██╔══╝  
╚██████╗██║  ██║██║  ██║██║  ██║   ██║   ██║  ██║███████╗ ╚████╔╝ ███████╗██║ ╚████║╚██████╔╝███████╗
 ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝╚══════╝  ╚═══╝  ╚══════╝╚═╝  ╚═══╝ ╚═════╝ ╚══════╝
                                                                                                       

`)

	//Display welcome message
	fmt.Println(purple+"Version:"+reset, version+"\n")

	fmt.Println(purple + "chartRevenue: Test Botify BQL.\n" + reset)
	fmt.Println(purple + "Use it as a template for your Botify integration needs.\n" + reset)
	fmt.Println(purple + "BQL tests performed in this version.\n" + reset)
	fmt.Println(checkmark + green + bold + " Revenue (YTD/monthly)" + reset)
	fmt.Println(checkmark + green + bold + " Visits (YTD/monthly)" + reset)
	fmt.Println(checkmark + green + bold + " Transactions (YTD/monthly)" + reset)
	fmt.Println(checkmark + green + bold + " (Computed) Average transaction value" + reset)
}

// Display the seperator
func displaySeparator() {
	block := "█"
	fmt.Println()

	for i := 0; i < 130; i++ {
		fmt.Print(block)
	}

	fmt.Println()
}

// Function to format an integer with comma separation
func formatWithCommas(n int) string {
	s := strconv.Itoa(n)
	nLen := len(s)
	if nLen <= 3 {
		return s
	}

	var result strings.Builder
	commaOffset := nLen % 3
	if commaOffset > 0 {
		result.WriteString(s[:commaOffset])
		if nLen > commaOffset {
			result.WriteString(",")
		}
	}

	for i := commaOffset; i < nLen; i += 3 {
		result.WriteString(s[i : i+3])
		if i+3 < nLen {
			result.WriteString(",")
		}
	}

	return result.String()
}

// Function to clear the screen
func clearScreen() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}
