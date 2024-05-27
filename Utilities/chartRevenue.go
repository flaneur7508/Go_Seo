// chartRevenue: Test Botify APIs
// Analysis based on 1MM URL maximum
// Written by Jason Vicinanza

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// Used to identify which analytics tool is in use
type AnalyticsID struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Date        string      `json:"date"`
	Timestamped bool        `json:"timestamped"`
	DateStart   string      `json:"date_start"`
	DateEnd     string      `json:"date_end"`
	GenericName interface{} `json:"generic_name"`
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
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Subtype        string      `json:"subtype"`
	Multiple       bool        `json:"multiple"`
	Permissions    []string    `json:"permissions"`
	Optional       bool        `json:"optional"`
	Kind           string      `json:"kind"`
	GlobalField    string      `json:"global_field"`
	DiffReturnType interface{} `json:"diff_return_type"`
	ApiOnly        bool        `json:"api_only"`
	Meta           meta        `json:"meta"`
	Suggestion     bool        `json:"suggestion"`
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
	Results  []Result    `json:"results"`
	Previous interface{} `json:"previous"`
	Next     string      `json:"next"`
	Page     int         `json:"page"`
	Size     int         `json:"size"`
}

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

	displaySeparator()

	// Revenue for the last 12 months
	seoRevenue()

	displaySeparator()

	// Visits for the last 12 months
	//seoVisits()

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
	// Define array to store startMthDate and endMthDate separately
	startMthDates := make([]string, 0)
	endMthDates := make([]string, 0)
	// Populate the array with string versions of the date ready for use in the BQL
	for _, dateRange := range dateRanges.MonthlyRanges {
		startMthDate := dateRange[0].Format("20060102")
		endMthDate := dateRange[1].Format("20060102")
		startMthDates = append(startMthDates, startMthDate)
		endMthDates = append(endMthDates, endMthDate)
	}

	// Format the YTD range ready for use in the BQL
	startYTDDate := dateRanges.YTDRange[0].Format("20060102")
	endYTDDate := dateRanges.YTDRange[1].Format("20060102")

	// Get the revenue data
	getRevenueData(analyticsID, startYTDDate, endYTDDate, startMthDates, endMthDates)
}

// Get the revenue, transactions and visits data
func getRevenueData(analyticsID string, startYTDDate string, endYTDDate string, startMthDates []string, endMthDates []string) {
	// Define the revenue endpoint
	var urlAPIRevenueData string
	if analyticsID == "visits.dip" {
		urlAPIRevenueData = "https://api.botify.com/v1/projects/" + orgName + "/" + projectName + "/collections/conversion.dip"
	} else {
		urlAPIRevenueData = "https://api.botify.com/v1/projects/" + orgName + "/" + projectName + "/collections/conversion"
	}

	//fmt.Println(bold+"\nRevenue data end point:"+reset, urlAPIRevenueData)
	req, errorCheck := http.NewRequest("GET", urlAPIRevenueData, nil)

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
		log.Fatal(red+"\nError. getRevenueData. Cannot create request: "+reset, errorCheck)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read the response body
	responseData, errorCheck := ioutil.ReadAll(resp.Body)
	if errorCheck != nil {
		log.Fatal(red+"Error. getRevenueData. Cannot read response body: "+reset, errorCheck)
		os.Exit(1)
	}

	// Unmarshal the JSON data into the struct
	var transRevIDs transRevID
	if err := json.Unmarshal(responseData, &transRevIDs); err != nil {
		log.Fatal(red+"Error. getRevenueData. Cannot unmarshall the JSON: "+reset, err)
		os.Exit(1)
	}

	// Get YTD insights
	fmt.Println(bold + "\nYTD organic insights" + reset)
	executeRevenueBQL(analyticsID, startYTDDate, endYTDDate)

	// Get monthly insights
	fmt.Println(bold + "\nMonthly organic insights" + reset)
	for i := range startMthDates {
		executeRevenueBQL(analyticsID, startMthDates[i], endMthDates[i])
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

func executeRevenueBQL(analyticsID string, startYTDDate string, endYTDDate string) {

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
	}`, analyticsID, startYTDDate, endYTDDate)

	// Define the URL
	url := fmt.Sprintf("https://api.botify.com/v1/projects/%s/%s/query", orgName, projectName)
	//fmt.Println("End point:", url, "\n")

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
		return
	}

	// Unmarshal the JSON data into the struct
	var response Response
	err := json.Unmarshal(responseData, &response)
	if err != nil {
		log.Fatalf("Error. executeRevenueBQL. Cannot unmarshal the JSON: %v", err)
	}

	// Check if any data has been returned from the API. Count the number of elements in the Results array
	responseCount := len(response.Results)

	if responseCount == 0 {
		fmt.Println(red + "Error. executeRevenueBQL. Cannot get Revenue & Visits data. Ensure the selected project is using GA4." + reset)
	} else {
		// Cast the float64 values as ints
		ytdMetricsTransactions := int(response.Results[0].Metrics[0])
		ytdMetricsRevenue := int(response.Results[0].Metrics[1])
		ytdMetricsVisits := int(response.Results[0].Metrics[2])
		fmt.Printf(green+"Start: %s End: %s\n"+reset, startYTDDate, endYTDDate)
		// Include commas in the display integer
		formattedTransactions := formatWithCommas(ytdMetricsTransactions)
		fmt.Println("No. transactions", formattedTransactions)
		formattedRevenue := formatWithCommas(ytdMetricsRevenue)
		fmt.Println("Total revenue", formattedRevenue)
		// Calculate the average transaction value
		avgTransactionValue := ytdMetricsRevenue / ytdMetricsTransactions
		fmt.Println("Average transaction value", avgTransactionValue)
		formattedVisits := formatWithCommas(ytdMetricsVisits)
		fmt.Println("No. of visits", formattedVisits)

		fmt.Println("\n")

	}

	/*
		// Copy the BQL to the clipboard for pasting into Postman
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(bqlRevTrans)
		err = cmd.Run()
		if err != nil {
			fmt.Println("Error. executeRevenueBQL. Cannot copy BQL to clipboard:", err)
			return
		}
	*/

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
