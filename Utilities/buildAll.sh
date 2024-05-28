#!/bin/bash

clear
echo "** Building Go_SEO utilities"
echo
echo - building folderCount.go
go build folderCount.go
echo - building paramCount.go
go build paramCount.go
echo - building listAnalysis.go
go build listAnalysis.go
echo - building listURLs.go
go build listURLs.go
echo - building segmentifyLite.go
go build segmentifyLite.go
echo - building apiTester.go
go build apiTester.go
echo - building bqlTester.go
go build bqlTester.go
echo - building chartRevenue.go
go build chartRevenue.go
cd botifyBotLite
echo - building botifyBotLite.go
go build botifyBotLite.go
echo
echo "** Building Go_SEO utilities - Done!"
#echo
#echo "** Packaging binaries"
cd /Users/jason/documents/go/go_seo/utilities
tar -cvf Go_Seo_macOS.tar ./folderCount ./paramCount ./listAnalysis ./listURLs ./segmentifyLite ./apiTester ./bqlTester ./chartRevenue botifyBotLite
#echo
#echo "** Packaging binaries - Done! - see Go_Seo_macOS.tar"
#echo





