//segmentifyLite
//Written by Jason Vicinanza

//To run this:
//go run segmentifyLite.go org_name project_name
//Example: go run segmentifyLite.go jason-org jason-project-name (with complier)
//Example: segmentifyLite.go jason-org jason-project-name (with executable)
//Remember to use your own api_key

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Version
var version = "v0.1"

// Specify your Botify API token here
var botify_api_token = "c1e6c5ab4a8dc6a16620fd0a885dd4bee7647205"

// Colours
var purple = "\033[0;35m"
var red = "\033[0;31m"
var bold = "\033[1m"
var reset = "\033[0m"

// Default input and output files
var inputFilename = "siteurlsExport.csv"
var outputFilename = "segment.txt"

// Unicode escape sequence for the checkmark symbol
var checkmark = "\u2713"

// Maximum No. of pages to export. 300 = 300k etc.
var maxURLsToExport = 300

// Percentage threshold for level 1 folders
var thresholdPercent = 0.05

// Boolean to signal if SFCC has been detected
var sfccDetected bool = false

// Boolean to signal if Shopify has been detected
var shopifyDetected bool = false

// Number of forward-slashes in the URL to count in order to identify the folder level
// 4 = level 1
// 5 = level 2
var slashCountL1 = 4
var slashCountL2 = 5

type botifyResponse struct {
	Next     string      `json:"next"`
	Previous interface{} `json:"previous"`
	Count    int         `json:"count"`
	Results  []struct {
		Slug string `json:"slug"`
	} `json:"results"`
	Page int `json:"page"`
	Size int `json:"size"`
}

// Define a struct to hold text value and its associated count
type FolderCount struct {
	Text  string
	Count int
}

// Implement sorting interface for FolderCount slice
type ByCount []FolderCount

func (a ByCount) Len() int           { return len(a) }
func (a ByCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCount) Less(i, j int) bool { return a[i].Count > a[j].Count }

func main() {

	clearScreen()

	displayBanner()

	//Display welcome message
	fmt.Println(purple + "\nsegmentifyLite: Fast segmentation regex generation\n" + reset)
	fmt.Println(purple+"Version:"+reset, version, "\n")

	//Generate the list of URLs
	urlExport()

	//Level1 folders
	//Get the threshold. Use the level 1 slashCount
	largestFolderSize, thresholdValueL1 := levelThreshold(inputFilename, slashCountL1)
	fmt.Printf(purple + "Calculating level 1 folder threshold\n" + reset)
	fmt.Printf("Largest level 1 folder size found is %d URLs\n", largestFolderSize)
	fmt.Printf("Threshold folder size: %d\n", thresholdValueL1)

	//generate the regex
	segmentLevel1(thresholdValueL1)

	//Level2 folders
	//Get the threshold. Use the level 2 slashCount
	largestFolderSize, thresholdValueL2 := levelThreshold(inputFilename, slashCountL2)
	fmt.Printf(purple + "\nCalculating level 2 folder threshold\n" + reset)
	fmt.Printf("Largest level 2 folder size found is %d URLs\n", largestFolderSize)
	fmt.Printf("Threshold folder size: %d\n", thresholdValueL2)

	//Level2 folders
	segmentLevel2(thresholdValueL2)

	//Subdomains
	subDomains()

	//Parameter keys
	parameterKeys()

	//Parameter keys utilization
	parameterUsage()

	//No. of parameter keys
	noOfParameters()

	//No. of folders
	noOfFolders()

	// Salesforce Commerce Cloud if detected
	if !sfccDetected {
		sfccURLs()
	}

	// Shopify if detected
	shopifyURLs()

	//It's done! segmentifyList has left the building
	fmt.Println(purple+"Your regex can be found in:", outputFilename+reset)
	fmt.Println(purple + "Regex generation complete" + reset)
}

// Use the API to get the first 300k URLs and export them to a file
func urlExport() {

	//Get the command line arguments for the org and project name
	if len(os.Args) < 3 {
		fmt.Println(red + "Error. Please provide the organisation, project name as line arguments")
		os.Exit(1)
	}
	orgName := os.Args[1]
	projectName := os.Args[2]

	//Get the last analysis slug
	url := fmt.Sprintf("https://api.botify.com/v1/analyses/%s/%s?page=1&only_success=true", orgName, projectName)

	req, errorCheck := http.NewRequest("GET", url, nil)
	if errorCheck != nil {
		log.Fatal("Error creating request: "+reset, errorCheck)
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "token "+botify_api_token)

	res, errorCheck := http.DefaultClient.Do(req)
	if errorCheck != nil {
		log.Fatal(red+"Error. Check your network connection: "+reset, errorCheck)
	}
	defer res.Body.Close()

	responseData, errorCheck := ioutil.ReadAll(res.Body)
	if errorCheck != nil {
		log.Fatal(red+"Error reading response body: "+reset, errorCheck)
		os.Exit(1)
	}

	var responseObject botifyResponse
	errorCheck = json.Unmarshal(responseData, &responseObject)

	if errorCheck != nil {
		log.Fatal(red+"Error. Cnnot unmarshall JSON: "+reset, errorCheck)
		os.Exit(1)
	}

	//Display an error if no crawls found
	if responseObject.Count == 0 {
		fmt.Println(red + "Error. Invalid crawl or no crawls found in the project")
		os.Exit(1)
	}

	//Display the welcome message
	fmt.Println(purple + "Exporting URLs" + reset)

	//Create a file for writing
	file, errorCheck := os.Create(inputFilename)
	if errorCheck != nil {
		fmt.Println(red+"Error creating file: "+reset, errorCheck)
		os.Exit(1)
	}
	defer file.Close()

	//Initialize total count
	totalCount := 0
	fmt.Println("Maximum No. of URLs to be exported is", maxURLsToExport, "k")
	fmt.Println("Organisation Name:", orgName)
	fmt.Println("Project Name:", projectName)
	fmt.Println("Latest analysis Slug:", responseObject.Results[0].Slug)
	analysisSlug := responseObject.Results[0].Slug
	urlEndpoint := fmt.Sprintf("https://api.botify.com/v1/analyses/%s/%s/%s/", orgName, projectName, analysisSlug)
	fmt.Println("End point:", urlEndpoint, "\n")

	//Iterate through pages 1 through to the maximum no of pages defined by maxURLsToExport
	//Each page returns 1000 URLs
	for page := 1; page <= maxURLsToExport; page++ {

		url := fmt.Sprintf("https://api.botify.com/v1/analyses/%s/%s/%s/urls?area=current&page=%d&size=1000", orgName, projectName, analysisSlug, page)

		payload := strings.NewReader("{\"fields\":[\"url\"]}")

		req, _ := http.NewRequest("POST", url, payload)

		req.Header.Add("accept", "application/json")
		req.Header.Add("content-type", "application/json")
		req.Header.Add("Authorization", "token "+botify_api_token)

		res, errorCheck := http.DefaultClient.Do(req)
		if errorCheck != nil {
			fmt.Println(red+"Error. Cannot connect to the API: "+reset, errorCheck)
			os.Exit(1)
		}
		defer res.Body.Close()

		//Decode JSON response
		var response map[string]interface{}
		if errorCheck := json.NewDecoder(res.Body).Decode(&response); errorCheck != nil {
			fmt.Println(red+"Error. Cannot decode JSON: "+reset, errorCheck)
			os.Exit(1)
		}

		//Extract URLs from the "results" key
		results, ok := response["results"].([]interface{})
		if !ok {
			fmt.Println(red + "Error. Results not found in response. Check the specified organisation and project")
			os.Exit(1)
		}

		//Write URLs to the file
		count := 0
		for _, result := range results {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if url, ok := resultMap["url"].(string); ok {
					// Check if SFCC is used. This bool us used to deterline if the SFCC regex is generated
					if strings.Contains(url, "/demandware/") {
						sfccDetected = true // Set sfccDetected to true if the condition is met
					}
					// Check if Shopify is used. This bool us used to deterline if the Shopify regex is generated
					if strings.Contains(url, "/collections/") && strings.Contains(url, "/products/") {
						shopifyDetected = true
					}
					if _, errorCheck := file.WriteString(url + "\n"); errorCheck != nil {
						fmt.Println(red+"Error. Cannot write to file: "+reset, errorCheck)
						os.Exit(1)
					}
					count++
					totalCount++
					if count%10 == 0 {
						fmt.Print("#") //Print "#" every 10 URLs. Used as a progress indicator
					}
				}
			}
		}

		//If there are no more URLS export exit the function
		if count == 0 {
			//Print total number of URLs saved
			fmt.Printf("\nTotal URLs exported: %d\n", totalCount)
			if sfccDetected {
				fmt.Printf(bold + "\nNote: Salesforce Commerce Cloud has been detected. Regex will be generated\n" + reset)
			}
			if shopifyDetected {
				fmt.Printf(bold + "\nNote: Shopify has been detected. Regex will be generated\n" + reset)
			}
			fmt.Println(purple + "\nURL Extract complete. Generating regex...\n" + reset)
			// Check if SFCC is used. This bool us used to deterline if SFCC regex is generated
			break
		}

		//Max. number of URLs (200k) has been reached
		if totalCount > 190000 {
			fmt.Printf("\n\nExport limit of %d URLs reached. Generating regex...\n\n", totalCount)
			break
		}

		fmt.Printf("\nPage %d: %d URLs exported\n", page, count)
	}

}

// Regex for level 1 folders
func segmentLevel1(thresholdValueL1 int) {

	//Open the input file
	file, errorCheck := os.Open(inputFilename)
	if errorCheck != nil {
		fmt.Printf(red+"segmentiftyLite. Error. Cannot open input file: %v\n "+reset, errorCheck)
		os.Exit(1)
	}
	defer file.Close()

	//Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	//Map to keep track of counts of unique values
	FolderCounts := make(map[string]int)

	//Variable to keep track of the total number of records processed
	totalRecords := 0

	//Counter to track the number of records scanned
	recordCounter := 0

	//Counter to track the number of folders excluded from the regex
	noFoldersExcludedL1 := 0

	//Display welcome message
	fmt.Println(purple + "\nFirst level folders" + reset)
	fmt.Printf("Folders with less than %d URLs will be excluded\n", thresholdValueL1)

	//Iterate through each line in the file
	for scanner.Scan() {
		line := scanner.Text()
		totalRecords++
		recordCounter++

		//Display a block for each 1000 records scanned
		if recordCounter%1000 == 0 {
			fmt.Print("#")
		}

		//Check if the line contains a quotation mark, if yes, skip to the next line
		if strings.Contains(line, "\"") {
			continue
		}

		//Split the line into substrings using a forward-slash as delimiter
		parts := strings.Split(line, "/")

		//Check if there are at least 4 parts in the line
		if len(parts) >= 4 {
			//Extract the text between the third and fourth forward-slashes
			text := strings.Join(parts[:4], "/")

			//Trim any leading or trailing whitespace
			text = strings.TrimSpace(text)

			//Update the count for this value if it's not empty
			if text != "" {
				FolderCounts[text]++
			}
		}
	}

	//Subtract 2 in order to account for the two header records which are defaults in Botify URL extracts
	totalRecords -= 2

	fmt.Printf("\n")

	//Create a slice to hold FolderCount structs
	var sortedCounts []FolderCount

	//Populate the slice with data from the map
	for folderName, count := range FolderCounts {
		if count > thresholdValueL1 {
			sortedCounts = append(sortedCounts, FolderCount{folderName, count})
		} else {
			// Count the number of folders excluded
			noFoldersExcludedL1++
		}
	}

	//Sort the slice based on counts
	sort.Sort(ByCount(sortedCounts))

	//Display the counts for each unique value
	for _, folderValueCount := range sortedCounts {
		fmt.Printf("%s (URLs: %d)\n", folderValueCount.Text, folderValueCount.Count)
	}

	fmt.Printf("\nNo. of level 1 folders excluded %d\n", noFoldersExcludedL1)

	//Open the output file for writing
	//Always create the file.
	outputFile, errorCheck := os.Create(outputFilename)
	if errorCheck != nil {
		fmt.Printf(red+"segment1stLevel. Error. Cannot create output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}
	defer outputFile.Close()

	//Create a writer to write to the output file
	writer := bufio.NewWriter(outputFile)

	//Write the header lines
	// Get the user's local time zone for the header
	userLocation, errorCheck := time.LoadLocation("") // Load the default local time zone
	if errorCheck != nil {
		fmt.Println("Error loading user's location:", errorCheck)
		return
	}
	// Get the current date and time in the user's local time zone
	currentTime := time.Now().In(userLocation)

	_, errorCheck = writer.WriteString(fmt.Sprintf("# Regex made with Go_SEO/segmentifyLite %s\n", version))
	_, errorCheck = writer.WriteString(fmt.Sprintf("# Generated on %s\n", currentTime.Format(time.RFC1123)))

	// Start of regex
	_, errorCheck = writer.WriteString(fmt.Sprintf("\n[segment:sl_level1_Folders]\n@Home\npath /\n\n"))

	if errorCheck != nil {
		fmt.Printf(red+"segment1stLevel. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Write the regex
	for _, folderValueCount := range sortedCounts {
		if folderValueCount.Text != "" {
			//Extract the text between the third and fourth forward-slashes
			parts := strings.SplitN(folderValueCount.Text, "/", 4)
			if len(parts) >= 4 && parts[3] != "" {
				folderLabel := parts[3] //Extract the text between the third and fourth forward-slashes
				_, errorCheck := writer.WriteString(fmt.Sprintf("@%s\nurl *%s/*\n\n", folderLabel, folderValueCount.Text))

				if errorCheck != nil {
					fmt.Printf(red+"segment1stLevel. Error. Cannot write to output file: %v\n"+reset, errorCheck)
					os.Exit(1)
				}
			}
		}
	}

	//Write the footer lines\
	_, errorCheck = writer.WriteString("@~Other\npath /*\n# ----End of level1Folders Segment----\n")
	if errorCheck != nil {
		fmt.Printf(red+"segment1stLevel. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Insert the number of URLs found in each folder as comments
	_, errorCheck = writer.WriteString("\n# ----Level 1 Folder URL analysis----\n")
	for _, folderValueCount := range sortedCounts {
		_, errorCheck := writer.WriteString(fmt.Sprintf("# --%s (URLs found: %d)\n", folderValueCount.Text, folderValueCount.Count))
		if errorCheck != nil {
			fmt.Printf(red+"segment1stLevel. Error. Cannot write to output file: %v\n"+reset, errorCheck)
			os.Exit(1)
		}
	}

	//Flush the writer to ensure all data is written to the file
	errorCheck = writer.Flush()
	if errorCheck != nil {
		fmt.Printf(red+"segment1stLevel. Error. Cannot flush writer: %v\n", errorCheck)
		os.Exit(1)
	}

	//Check for any errors during scanning
	if errorCheck := scanner.Err(); errorCheck != nil {
		fmt.Printf(red+"segment1stLevel. Error. Cannot scan input file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}
}

// Regex for level 2 folders
func segmentLevel2(thresholdValueL2 int) {

	//Open the input file
	file, errorCheck := os.Open(inputFilename)
	if errorCheck != nil {
		os.Exit(1)
	}
	defer file.Close()

	//Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	//Map to keep track of counts of unique values
	FolderCounts := make(map[string]int)

	//Variable to keep track of the total number of records processed
	totalRecords := 0

	//Counter to track the number of records scanned
	recordCounter := 0

	//Counter to track the number of folders excluded from the regex
	noFoldersExcludedL2 := 0

	//Display welcome message
	fmt.Println(purple + "\nSecond level folders" + reset)
	fmt.Printf("Folders with less than %d URLs will be excluded\n", thresholdValueL2)

	//Iterate through each line in the file
	for scanner.Scan() {
		line := scanner.Text()
		totalRecords++
		recordCounter++

		//Display a block for each 1000 records scanned
		if recordCounter%1000 == 0 {
			fmt.Print("#")
		}

		//Check if the line contains a quotation mark, if yes, skip to the next line
		if strings.Contains(line, "\"") {
			continue
		}

		//Split the line into substrings using a forward-slash as delimiter
		parts := strings.Split(line, "/")

		//Check if there are at least 4 parts in the line
		if len(parts) >= 5 {
			//Extract the text between the third and fourth forward-slashes
			text := strings.Join(parts[:5], "/")

			//Trim any leading or trailing whitespace
			text = strings.TrimSpace(text)

			//Update the count for this value if it's not empty
			if text != "" {
				FolderCounts[text]++
			}
		}
	}

	//Subtract 2 in order to account for the two header records which are defaults in Botify URL extracts
	totalRecords -= 2

	fmt.Printf("\n")

	//Create a slice to hold FolderCount structs
	var sortedCounts []FolderCount

	//Populate the slice with data from the map
	for folderName, count := range FolderCounts {
		if count > thresholdValueL2 {
			sortedCounts = append(sortedCounts, FolderCount{folderName, count})
		} else {
			// Count the number of folders excluded
			noFoldersExcludedL2++
		}
	}

	//Sort the slice based on counts
	sort.Sort(ByCount(sortedCounts))

	//Display the counts for each unique value
	for _, folderValueCount := range sortedCounts {
		fmt.Printf("%s (URLs: %d)\n", folderValueCount.Text, folderValueCount.Count)
	}

	fmt.Printf("\nNo. of level 2 folders excluded %d\n", noFoldersExcludedL2)

	//Open the file in append mode, create if it doesn't exist
	outputFile, errorCheck := os.OpenFile(outputFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if errorCheck != nil {
		panic(errorCheck)
	}
	defer outputFile.Close()

	//Create a writer to write to the output file
	writer := bufio.NewWriter(outputFile)

	//Write the header lines
	_, errorCheck = writer.WriteString(fmt.Sprintf("\n\n[segment:sl_level2_Folders]\n@Home\npath /\n\n"))

	if errorCheck != nil {
		fmt.Printf(red+"segment2ndLevel. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Write the regex
	for _, folderValueCount := range sortedCounts {
		if folderValueCount.Text != "" {
			//Extract the text between the third and fourth forward-slashes
			parts := strings.SplitN(folderValueCount.Text, "/", 4)
			if len(parts) >= 4 && parts[3] != "" {
				folderLabel := parts[3] //Extract the text between the third and fourth forward-slashes
				_, errorCheck := writer.WriteString(fmt.Sprintf("@%s\nurl *%s/*\n\n", folderLabel, folderValueCount.Text))
				if errorCheck != nil {
					fmt.Printf(red+"segment2ndLevel. Error. Cannot write to output file: %v\n"+reset, errorCheck)
					os.Exit(1)
				}
			}
		}
	}

	//Write the footer lines
	_, errorCheck = writer.WriteString("@~Other\npath /*\n# ----End of level2Folders Segment----\n")
	if errorCheck != nil {
		fmt.Printf(red+"segment2ndLevel. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Insert the number of URLs found in each folder as comments
	_, errorCheck = writer.WriteString("\n# ----Level 2 Folder URL analysis----\n")
	for _, folderValueCount := range sortedCounts {
		_, errorCheck := writer.WriteString(fmt.Sprintf("# --%s (URLs found: %d)\n", folderValueCount.Text, folderValueCount.Count))
		if errorCheck != nil {
			fmt.Printf(red+"segment1stLevel. Error. Cannot write to output file: %v\n"+reset, errorCheck)
			os.Exit(1)
		}
	}

	//Flush the writer to ensure all data is written to the file
	errorCheck = writer.Flush()
	if errorCheck != nil {
		fmt.Printf(red+"segment2ndLevel. Error. Cannot flush writer: %v\n", errorCheck)
		os.Exit(1)
	}

	//Check for any errors during scanning
	if errorCheck := scanner.Err(); errorCheck != nil {
		fmt.Printf(red+"segment2ndLevel. Error. Cannot scan input file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}
}

// Regex for subdomains
func subDomains() {

	//Open the input file
	file, errorCheck := os.Open(inputFilename)
	if errorCheck != nil {
		os.Exit(1)
	}
	defer file.Close()

	//Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	//Map to keep track of counts of unique values
	FolderCounts := make(map[string]int)

	//Variable to keep track of the total number of records processed
	totalRecords := 0

	//Counter to track the number of records scanned
	recordCounter := 0

	//Display welcome message
	fmt.Println(purple + "\nSubdomains" + reset)

	for scanner.Scan() {
		line := scanner.Text()
		totalRecords++
		recordCounter++

		//Display a block for each 1000 records scanned
		if recordCounter%1000 == 0 {
			fmt.Print("#")
		}

		//Check if the line contains a quotation mark, if yes, skip to the next line
		if strings.Contains(line, "\"") {
			continue
		}

		//Split the line into substrings using a forward-slash as delimiter
		parts := strings.Split(line, "/")
		//Check if there are at least 4 parts in the line
		if len(parts) >= 4 {
			//Extract the text between the third and fourth forward-slashes
			text := strings.Join(parts[:3], "/")

			//Trim any leading or trailing whitespace
			text = strings.TrimSpace(text)

			//Update the count for this value if it's not empty
			if text != "" {
				//Update the count for this value if it's not empty
				FolderCounts[text]++
			}
		}
	}

	//Subtract 2 in order to account for the two header records which are defaults in Botify URL extracts
	totalRecords -= 2

	fmt.Printf("\n")

	//Create a slice to hold FolderCount structs
	var sortedCounts []FolderCount

	//Populate the slice with data from the map
	for folderName, count := range FolderCounts {
		sortedCounts = append(sortedCounts, FolderCount{folderName, count})
	}

	//Sort the slice based on counts
	sort.Sort(ByCount(sortedCounts))

	//Display the counts for each unique value
	for _, folderValueCount := range sortedCounts {
		fmt.Printf("%s (URLs: %d)\n", folderValueCount.Text, folderValueCount.Count)
	}

	//Open the file in append mode, create if it doesn't exist
	outputFile, errorCheck := os.OpenFile(outputFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if errorCheck != nil {
		panic(errorCheck)
	}
	defer outputFile.Close()

	//Create a writer to write to the output file
	writer := bufio.NewWriter(outputFile)

	//Write the header lines
	_, errorCheck = writer.WriteString(fmt.Sprintf("\n\n[segment:sl_subdomains]\n@Home\npath /\n\n"))

	if errorCheck != nil {
		fmt.Printf(red+"subDomains. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Write the regex
	for _, folderValueCount := range sortedCounts {
		if folderValueCount.Text != "" {
			//Extract the text between the third and fourth forward-slashes
			parts := strings.SplitN(folderValueCount.Text, "/", 4)
			if len(parts) >= 3 && parts[2] != "" {
				folderLabel := parts[2] //Extract the text between the third and fourth forward-slashes
				_, errorCheck := writer.WriteString(fmt.Sprintf("@%s\nurl *%s/*\n\n", folderLabel, folderValueCount.Text))
				if errorCheck != nil {
					fmt.Printf(red+"subDomains. Error. Cannot write to output file: %v\n"+reset, errorCheck)
					os.Exit(1)
				}
			}
		}
	}

	//Write the footer lines
	_, errorCheck = writer.WriteString("@~Other\npath /*\n# ----End of subDomains Segment----\n")
	if errorCheck != nil {
		fmt.Printf(red+"subDomains. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Insert the number of URLs found in each folder as comments
	_, errorCheck = writer.WriteString("\n# ----subDomains Folder URL analysis----\n")
	for _, folderValueCount := range sortedCounts {
		_, errorCheck := writer.WriteString(fmt.Sprintf("# --%s (URLs found: %d)\n", folderValueCount.Text, folderValueCount.Count))
		if errorCheck != nil {
			fmt.Printf(red+"subDomains. Error. Cannot write to output file: %v\n"+reset, errorCheck)
			os.Exit(1)
		}
	}

	//Flush the writer to ensure all data is written to the file
	errorCheck = writer.Flush()
	if errorCheck != nil {
		fmt.Printf(red+"subDomains. Error. Cannot flush writer: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Check for any errors during scanning
	if errorCheck := scanner.Err(); errorCheck != nil {
		fmt.Printf(red+"subDomains. Error. Cannot scan input file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}
}

// Regex to identify which parameter keys are used
func parameterKeys() {

	//Open the input file
	file, errorCheck := os.Open(inputFilename)
	if errorCheck != nil {
		os.Exit(1)
	}
	defer file.Close()

	//Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	//Map to keep track of counts of unique values
	FolderCounts := make(map[string]int)

	//Variable to keep track of the total number of records processed
	totalRecords := 0

	//Counter to track the number of records scanned
	recordCounter := 0

	//Display welcome message
	fmt.Println(purple + "\nParameter keys" + reset)

	for scanner.Scan() {
		line := scanner.Text()
		totalRecords++
		recordCounter++

		//Display a block for each 1000 records scanned
		if recordCounter%1000 == 0 {
			fmt.Print("#")
		}

		//Check if the line contains a quotation mark, if yes, skip to the next line
		if strings.Contains(line, "\"") {
			continue
		}

		//Split the line into substrings using question mark as delimiter
		parts := strings.Split(line, "?")

		//Iterate over the parts after each question mark
		for _, part := range parts[1:] {
			//Find the index of the equals sign
			equalsIndex := strings.Index(part, "=")
			if equalsIndex != -1 {
				//Extract the text between the question mark and the equals sign
				text := part[:equalsIndex]

				//Trim any leading or trailing whitespace
				text = strings.TrimSpace(text)

				//Update the count for this value
				FolderCounts[text]++
			}
		}
	}

	//Subtract 2 in order to account for the two header records which are defaults in Botify URL extracts
	totalRecords -= 2

	fmt.Printf("\n")

	//Create a slice to hold FolderCount structs
	var sortedCounts []FolderCount

	//Populate the slice with data from the map
	for folderName, count := range FolderCounts {
		sortedCounts = append(sortedCounts, FolderCount{folderName, count})
	}

	//Sort the slice based on counts
	sort.Sort(ByCount(sortedCounts))

	//Display the counts for each unique value
	for _, folderValueCount := range sortedCounts {
		fmt.Printf("%s (URLs: %d)\n", folderValueCount.Text, folderValueCount.Count)
	}

	//Open the file in append mode, create if it doesn't exist
	outputFile, errorCheck := os.OpenFile(outputFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if errorCheck != nil {
		panic(errorCheck)
	}
	defer outputFile.Close()

	//Create a writer to write to the output file
	writer := bufio.NewWriter(outputFile)

	//Write the header lines
	//_, errorCheck = writer.WriteString(fmt.Sprintf("\n\n[segment:sl_parameterKeys]\n@Home\npath /\n\n"))
	_, errorCheck = writer.WriteString(fmt.Sprintf("\n\n[segment:sl_parameterKeys]\n"))

	if errorCheck != nil {
		fmt.Printf(red+"parameterKeys. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Write the regex
	for _, folderValueCount := range sortedCounts {
		_, errorCheck := writer.WriteString(fmt.Sprintf("@%s\nquery *%s=*\n\n", folderValueCount.Text, folderValueCount.Text))
		if errorCheck != nil {
			fmt.Printf(red+"parameterKeys. Error. Cannot write to output file: %v\n"+reset, errorCheck)
			os.Exit(1)
		}
	}

	//Write the footer lines
	_, errorCheck = writer.WriteString("@~Other\npath /*\n# ----End of parameterKeys Segment----\n")
	if errorCheck != nil {
		fmt.Printf(red+"parameterKeys. Error. Cannot write header to output file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Insert the number of URLs found in each folder as comments
	_, errorCheck = writer.WriteString("\n# ----parameterKeys URL analysis----\n")
	for _, folderValueCount := range sortedCounts {
		_, errorCheck := writer.WriteString(fmt.Sprintf("# --%s (URLs found: %d)\n", folderValueCount.Text, folderValueCount.Count))
		if errorCheck != nil {
			fmt.Printf(red+"parameterKeys. Error. Cannot write to output file: %v\n"+reset, errorCheck)
			os.Exit(1)
		}
	}

	//Flush the writer to ensure all data is written to the file
	errorCheck = writer.Flush()
	if errorCheck != nil {
		fmt.Printf(red+"parameterKeys. Error. Cannot flush writer: %v\n"+reset, errorCheck)
		os.Exit(1)
	}

	//Check for any errors during scanning
	if errorCheck := scanner.Err(); errorCheck != nil {
		fmt.Printf(red+"parameterKeys. Error. Canot scan input file: %v\n"+reset, errorCheck)
		os.Exit(1)
	}
}

// Regex to identify of a parameter key is used in the URL
func parameterUsage() {

	//Pages containing parameters
	paramaterUsageRegex := `

[segment:sl_parameter_Usage]
@Parameters
query *=*

@Clean
path /*
# ----End of sl_parameter_Usage----`

	//Parameter usage message
	fmt.Println(purple + "\nParameter usage" + reset)
	errParamaterUsage := insertStaticRegex(paramaterUsageRegex)
	if errParamaterUsage != nil {
		panic(errParamaterUsage)
	}

	//Finished
	fmt.Println("Done!", checkmark, "\n")

}

// Regex to count the number of parameters in the URL
func noOfParameters() {

	//Number of paramaters
	paramaterNoRegex := `


[segment:sl_no_Of_Parameters]
@Home
path /

@5_Parameters
query rx:=(.)+=(.)+=(.)+(.)+(.)+

@4_Parameters
query rx:=(.)+=(.)+=(.)+(.)+

@3_Parameters
query rx:=(.)+=(.)+=(.)+

@2_Parameters
query rx:=(.)+=(.)+

@1_Parameter
query rx:=(.)+

@~Other
path /*
# ----End of sl_no_Of_Parameters----`

	//No. of parameters message
	fmt.Println(purple + "Number of parameters" + reset)
	errParamaterNoRegex := insertStaticRegex(paramaterNoRegex)
	if errParamaterNoRegex != nil {
		panic(errParamaterNoRegex)
	}

	//Finished
	fmt.Println("Done!", checkmark, "\n")

}

// Regex to count the number of folders in the URL
func noOfFolders() {

	//Number of folders
	folderNoRegex := `


[segment:sl_no_Of_Folders]
@Home
path /

@Folders/5
path rx:^/[^/]+/[^/]+/[^/]+/[^/]+/[^/]+

@Folders/4
path rx:^/[^/]+/[^/]+/[^/]+/[^/]+

@Folders/3
path rx:^/[^/]+/[^/]+/[^/]+

@Folders/2
path rx:^/[^/]+/[^/]+

@Folders/1
path rx:^/[^/]+

@~Other
path /*
# ----End of sl_no_Of_Folders----`

	//No. of folders message
	fmt.Println(purple + "Number of folders" + reset)
	errFolderNoRegex := insertStaticRegex(folderNoRegex)
	if errFolderNoRegex != nil {
		panic(errFolderNoRegex)
	}

	//Finished
	fmt.Println("Done!", checkmark, "\n")

}

// SFCC Regex
func sfccURLs() {

	//SFCC
	sfccURLs := `


[segment:sl_sfcc_urls]
@Home
path /

@SFCC_URLs
path */demandware*

@~Other
path /*

# ----End of sl_sfcc_URLs----`

	// SFCC message
	fmt.Println(purple + "Salesforce Commerce Cloud (Demandware)" + reset)
	errSfccURLs := insertStaticRegex(sfccURLs)
	if errSfccURLs != nil {
		panic(errSfccURLs)
	}

	//Finished
	fmt.Println("Done!", checkmark, "\n")

}

// Shopify Regex
func shopifyURLs() {

	// Shopify
	shopifyURLs := `


[segment:sl_shopify]
@Home
path /

@PDP/Products/Variants
path */products/*
URL *variant=*

@PDP/Products
path */products/*

@PLP/Collections
path */collections/*

@Pages
path */pages/*

@~Other
path /*
# ----End of s_shopify_URLs----`

	// Shopify message
	fmt.Println(purple + "Shopify" + reset)
	errShopify := insertStaticRegex(shopifyURLs)
	if errShopify != nil {
		panic(errShopify)
	}

	//Finished
	fmt.Println("Done!", checkmark, "\n")

}

// Get the folder size threshold for level 1 & 2 folders
func levelThreshold(inputFilename string, slashCount int) (largestValueSize, fivePercentValue int) {
	// Open the input file
	file, errorCheck := os.Open(inputFilename)
	if errorCheck != nil {
		fmt.Printf("Error: Cannot open input file: %v\n", errorCheck)
		return 0, 0
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Map to keep track of counts of unique values
	FolderCounts := make(map[string]int)

	// Variable to keep track of the total number of records processed
	totalRecords := 0

	// Counter to track the number of records scanned
	recordCounter := 0

	// Iterate through each line in the file
	for scanner.Scan() {
		line := scanner.Text()
		totalRecords++
		recordCounter++

		// Check if the line contains a quotation mark, if yes, skip to the next line
		if strings.Contains(line, "\"") {
			continue
		}

		// Split the line into substrings using a forward-slash as delimiter
		parts := strings.Split(line, "/")

		// Check if there are at least slashCount parts in the URL
		// See slashCount variable declaration comments for more information
		if len(parts) >= slashCount {
			// Extract the text between the forward-slashes
			text := strings.Join(parts[:slashCount], "/")

			// Trim any leading or trailing whitespace
			text = strings.TrimSpace(text)

			// Update the count for this value if it's not empty
			if text != "" {
				FolderCounts[text]++
			}
		}
	}

	// Subtract 2 in order to account for the two header records which are defaults in URL extract
	totalRecords -= 2

	// Create a slice to hold FolderCount structs
	var sortedCounts []FolderCount

	// Populate the slice with data from the map
	for folderName, count := range FolderCounts {
		sortedCounts = append(sortedCounts, FolderCount{folderName, count})
	}

	// Sort the slice based on counts
	sort.Sort(ByCount(sortedCounts))

	// Get the largest value size
	if len(sortedCounts) > 0 {
		largestValueSize = sortedCounts[0].Count
	}

	// Calculate 5% of the largest value
	fivePercentValue = int(float64(largestValueSize) * thresholdPercent)

	return largestValueSize, fivePercentValue
}

// Write the static Regex to the segments file
func insertStaticRegex(regexText string) error {

	//Open the file in append mode, create if it doesn't exist
	outputFile, errorCheck := os.OpenFile(outputFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if errorCheck != nil {
		panic(errorCheck)
	}
	defer outputFile.Close()

	//Create a writer to write to the output file
	writer := bufio.NewWriter(outputFile)

	_, errorCheck = writer.WriteString(regexText)
	if errorCheck != nil {
		panic(errorCheck)
	}

	//Flush the writer to ensure all data is written to the file
	errorCheck = writer.Flush()
	if errorCheck != nil {
		fmt.Printf(red+"parameterUsage. Error. Cannot flush writer: %v\n"+reset, errorCheck)
		return errorCheck
	}

	return errorCheck
}

// Display the welcome banner
func displayBanner() {

	//ANSI escape code for Green
	green := "\033[0;32m"

	//Banner
	//https://patorjk.com/software/taag/#p=display&c=bash&f=ANSI%20Shadow&t=SegmentifyLite
	fmt.Println(green + `
███████╗███████╗ ██████╗ ███╗   ███╗███████╗███╗   ██╗████████╗██╗███████╗██╗   ██╗██╗     ██╗████████╗███████╗
██╔════╝██╔════╝██╔════╝ ████╗ ████║██╔════╝████╗  ██║╚══██╔══╝██║██╔════╝╚██╗ ██╔╝██║     ██║╚══██╔══╝██╔════╝
███████╗█████╗  ██║  ███╗██╔████╔██║█████╗  ██╔██╗ ██║   ██║   ██║█████╗   ╚████╔╝ ██║     ██║   ██║   █████╗
╚════██║██╔══╝  ██║   ██║██║╚██╔╝██║██╔══╝  ██║╚██╗██║   ██║   ██║██╔══╝    ╚██╔╝  ██║     ██║   ██║   ██╔══╝
███████║███████╗╚██████╔╝██║ ╚═╝ ██║███████╗██║ ╚████║   ██║   ██║██║        ██║   ███████╗██║   ██║   ███████╗
╚══════╝╚══════╝ ╚═════╝ ╚═╝     ╚═╝╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚═╝╚═╝        ╚═╝   ╚══════╝╚═╝   ╚═╝   ╚══════╝
`)
}

// Clear the screen
func clearScreen() {

	// Determine the appropriate command based on the operating system used
	var clearCmd string

	switch runtime.GOOS {
	case "windows":
		clearCmd = "cls"
	default:
		clearCmd = "clear"
	}

	cmd := exec.Command(clearCmd)
	cmd.Stdout = os.Stdout
	cmd.Run()
}